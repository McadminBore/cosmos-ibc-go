package types_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	cosmwasm "github.com/CosmWasm/wasmvm"
	wasmvmtypes "github.com/CosmWasm/wasmvm/types"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	wasmtesting "github.com/cosmos/ibc-go/modules/light-clients/08-wasm/testing"
	"github.com/cosmos/ibc-go/modules/light-clients/08-wasm/types"
	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	host "github.com/cosmos/ibc-go/v8/modules/core/24-host"
	"github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibctm "github.com/cosmos/ibc-go/v8/modules/light-clients/07-tendermint"
)

// var frozenHeight = clienttypes.NewHeight(0, 1)

// TestCheckSubstituteAndUpdateState only tests the interface to the contract, not the full logic of the contract.
func (suite *TypesTestSuite) TestCheckSubstituteAndUpdateStateGrandpa() {
	var (
		ok                                        bool
		subjectClientState, substituteClientState exported.ClientState
		subjectClientStore, substituteClientStore storetypes.KVStore
	)
	testCases := []struct {
		name    string
		setup   func()
		expPass bool
	}{
		{
			"success",
			func() {},
			true,
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupWasmGrandpaWithChannel()
			subjectClientState, ok = suite.chainA.App.GetIBCKeeper().ClientKeeper.GetClientState(suite.ctx, defaultWasmClientID)
			suite.Require().True(ok)
			subjectClientStore = suite.chainA.App.GetIBCKeeper().ClientKeeper.ClientStore(suite.ctx, defaultWasmClientID)

			substituteClientState, ok = suite.chainA.App.GetIBCKeeper().ClientKeeper.GetClientState(suite.ctx, defaultWasmClientID)
			suite.Require().True(ok)

			consensusStateData, err := base64.StdEncoding.DecodeString(suite.testData["consensus_state_data"])
			suite.Require().NoError(err)
			substituteConsensusState := types.ConsensusState{
				Data: consensusStateData,
			}

			substituteClientStore = suite.chainA.App.GetIBCKeeper().ClientKeeper.ClientStore(suite.ctx, "08-wasm-1")
			err = substituteClientState.Initialize(suite.ctx, suite.chainA.Codec, substituteClientStore, &substituteConsensusState)
			suite.Require().NoError(err)

			tc.setup()

			err = subjectClientState.CheckSubstituteAndUpdateState(
				suite.ctx,
				suite.chainA.Codec,
				subjectClientStore,
				substituteClientStore,
				substituteClientState,
			)
			if tc.expPass {
				suite.Require().NoError(err)

				// Verify that the substitute client state is in the subject client store
				clientStateBz := subjectClientStore.Get(host.ClientStateKey())
				suite.Require().NotEmpty(clientStateBz)
				newClientState := clienttypes.MustUnmarshalClientState(suite.chainA.Codec, clientStateBz)
				suite.Require().Equal(substituteClientState.GetLatestHeight(), newClientState.GetLatestHeight())
			} else {
				suite.Require().Error(err)
			}
		})
	}
}


func (suite *TypesTestSuite) TestCheckSubstituteAndUpdateState() {
	var substituteClientState exported.ClientState
	contractErr := errors.New("contract error")

	testCases := []struct {
		name     string
		malleate func()
		expErr   error
	}{
		{
			"success",
			func() {
				suite.mockVM.RegisterSudoCallback(
					types.CheckSubstituteAndUpdateStateMsg{},
					func(_ cosmwasm.Checksum, _ wasmvmtypes.Env, sudoMsg []byte, store cosmwasm.KVStore, _ cosmwasm.GoAPI, _ cosmwasm.Querier, _ cosmwasm.GasMeter, _ uint64, _ wasmvmtypes.UFraction) (*wasmvmtypes.Response, uint64, error) {
						var payload types.SudoMsg
						err := json.Unmarshal(sudoMsg, &payload)
						suite.Require().NoError(err)

						suite.Require().NotNil(payload.CheckSubstituteAndUpdateState)
						suite.Require().Nil(payload.UpdateState)
						suite.Require().Nil(payload.UpdateStateOnMisbehaviour)
						suite.Require().Nil(payload.VerifyMembership)
						suite.Require().Nil(payload.VerifyNonMembership)
						suite.Require().Nil(payload.VerifyUpgradeAndUpdateState)

						bz, err := json.Marshal(types.EmptyResult{})
						suite.Require().NoError(err)

						return &wasmvmtypes.Response{Data: bz}, types.DefaultGasUsed, nil
					},
				)
			},
			nil,
		},
		{
			"failure: invalid substitute client state",
			func() {
				substituteClientState = &ibctm.ClientState{}
			},
			clienttypes.ErrInvalidClient,
		},
		{
			"failure: contract returns error",
			func() {
				suite.mockVM.RegisterSudoCallback(
					types.CheckSubstituteAndUpdateStateMsg{},
					func(_ cosmwasm.Checksum, _ wasmvmtypes.Env, sudoMsg []byte, store cosmwasm.KVStore, _ cosmwasm.GoAPI, _ cosmwasm.Querier, _ cosmwasm.GasMeter, _ uint64, _ wasmvmtypes.UFraction) (*wasmvmtypes.Response, uint64, error) {
						return nil, types.DefaultGasUsed, contractErr
					},
				)
			},
			contractErr,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupWasmWithMockVM()

			endpointA := wasmtesting.NewWasmEndpoint(suite.chainA)
			err := endpointA.CreateClient()
			suite.Require().NoError(err)

			subjectClientStore := suite.chainA.App.GetIBCKeeper().ClientKeeper.ClientStore(suite.chainA.GetContext(), endpointA.ClientID)
			subjectClientState := endpointA.GetClientState()

			endpointB := wasmtesting.NewWasmEndpoint(suite.chainA)
			err = endpointB.CreateClient()
			suite.Require().NoError(err)
			substituteClientState = endpointB.GetClientState()
			substituteClientStore := suite.chainA.App.GetIBCKeeper().ClientKeeper.ClientStore(suite.chainA.GetContext(), endpointB.ClientID)

			tc.malleate()

			err = subjectClientState.CheckSubstituteAndUpdateState(
				suite.chainA.GetContext(),
				suite.chainA.Codec,
				subjectClientStore,
				substituteClientStore,
				substituteClientState,
			)

			expPass := tc.expErr == nil
			if expPass {
				suite.Require().NoError(tc.expErr)
			} else {
				suite.Require().ErrorIs(err, tc.expErr)
			}
		})
	}
}


func GetProcessedHeight(clientStore storetypes.KVStore, height exported.Height) (uint64, bool) {
	key := ibctm.ProcessedHeightKey(height)
	bz := clientStore.Get(key)
	if len(bz) == 0 {
		return 0, false
	}

	return sdk.BigEndianToUint64(bz), true
}
