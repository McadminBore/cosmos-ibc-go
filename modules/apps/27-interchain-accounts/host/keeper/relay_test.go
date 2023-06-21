package keeper_test

import (
	"github.com/cosmos/gogoproto/proto"

	sdkmath "cosmossdk.io/math"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	disttypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/host/types"
	icatypes "github.com/cosmos/ibc-go/v7/modules/apps/27-interchain-accounts/types"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ibctesting "github.com/cosmos/ibc-go/v7/testing"
)

func (s *KeeperTestSuite) TestOnRecvPacket() {
	var (
		path       *ibctesting.Path
		packetData []byte
	)

	testCases := []struct {
		msg      string
		malleate func()
		expPass  bool
	}{
		{
			"interchain account successfully executes an arbitrary message type using the * (allow all message types) param",
			func() {
				interchainAccountAddr, found := s.chainB.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(s.chainB.GetContext(), ibctesting.FirstConnectionID, path.EndpointA.ChannelConfig.PortID)
				s.Require().True(found)

				// Populate the gov keeper in advance with an active proposal
				testProposal := &govtypes.TextProposal{
					Title:       "IBC Gov Proposal",
					Description: "tokens for all!",
				}

				proposalMsg, err := govv1.NewLegacyContent(testProposal, interchainAccountAddr)
				s.Require().NoError(err)

				proposal, err := govv1.NewProposal([]sdk.Msg{proposalMsg}, govtypes.DefaultStartingProposalID, s.chainA.GetContext().BlockTime(), s.chainA.GetContext().BlockTime(), "test proposal", "title", "Description", sdk.AccAddress(interchainAccountAddr))
				s.Require().NoError(err)

				s.chainB.GetSimApp().GovKeeper.SetProposal(s.chainB.GetContext(), proposal)
				s.chainB.GetSimApp().GovKeeper.ActivateVotingPeriod(s.chainB.GetContext(), proposal)

				msg := &govtypes.MsgVote{
					ProposalId: govtypes.DefaultStartingProposalID,
					Voter:      interchainAccountAddr,
					Option:     govtypes.OptionYes,
				}

				data, err := icatypes.SerializeCosmosTx(s.chainA.GetSimApp().AppCodec(), []proto.Message{msg})
				s.Require().NoError(err)

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.EXECUTE_TX,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()

				params := types.NewParams(true, []string{"*"})
				s.chainB.GetSimApp().ICAHostKeeper.SetParams(s.chainB.GetContext(), params)
			},
			true,
		},
		{
			"interchain account successfully executes banktypes.MsgSend",
			func() {
				interchainAccountAddr, found := s.chainB.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(s.chainB.GetContext(), ibctesting.FirstConnectionID, path.EndpointA.ChannelConfig.PortID)
				s.Require().True(found)

				msg := &banktypes.MsgSend{
					FromAddress: interchainAccountAddr,
					ToAddress:   s.chainB.SenderAccount.GetAddress().String(),
					Amount:      sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100))),
				}

				data, err := icatypes.SerializeCosmosTx(s.chainA.GetSimApp().AppCodec(), []proto.Message{msg})
				s.Require().NoError(err)

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.EXECUTE_TX,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()

				params := types.NewParams(true, []string{sdk.MsgTypeURL(msg)})
				s.chainB.GetSimApp().ICAHostKeeper.SetParams(s.chainB.GetContext(), params)
			},
			true,
		},
		{
			"interchain account successfully executes stakingtypes.MsgDelegate",
			func() {
				interchainAccountAddr, found := s.chainB.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(s.chainB.GetContext(), ibctesting.FirstConnectionID, path.EndpointA.ChannelConfig.PortID)
				s.Require().True(found)

				validatorAddr := (sdk.ValAddress)(s.chainB.Vals.Validators[0].Address)
				msg := &stakingtypes.MsgDelegate{
					DelegatorAddress: interchainAccountAddr,
					ValidatorAddress: validatorAddr.String(),
					Amount:           sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(5000)),
				}

				data, err := icatypes.SerializeCosmosTx(s.chainA.GetSimApp().AppCodec(), []proto.Message{msg})
				s.Require().NoError(err)

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.EXECUTE_TX,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()

				params := types.NewParams(true, []string{sdk.MsgTypeURL(msg)})
				s.chainB.GetSimApp().ICAHostKeeper.SetParams(s.chainB.GetContext(), params)
			},
			true,
		},
		{
			"interchain account successfully executes stakingtypes.MsgDelegate and stakingtypes.MsgUndelegate sequentially",
			func() {
				interchainAccountAddr, found := s.chainB.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(s.chainB.GetContext(), ibctesting.FirstConnectionID, path.EndpointA.ChannelConfig.PortID)
				s.Require().True(found)

				validatorAddr := (sdk.ValAddress)(s.chainB.Vals.Validators[0].Address)
				msgDelegate := &stakingtypes.MsgDelegate{
					DelegatorAddress: interchainAccountAddr,
					ValidatorAddress: validatorAddr.String(),
					Amount:           sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(5000)),
				}

				msgUndelegate := &stakingtypes.MsgUndelegate{
					DelegatorAddress: interchainAccountAddr,
					ValidatorAddress: validatorAddr.String(),
					Amount:           sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(5000)),
				}

				data, err := icatypes.SerializeCosmosTx(s.chainA.GetSimApp().AppCodec(), []proto.Message{msgDelegate, msgUndelegate})
				s.Require().NoError(err)

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.EXECUTE_TX,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()

				params := types.NewParams(true, []string{sdk.MsgTypeURL(msgDelegate), sdk.MsgTypeURL(msgUndelegate)})
				s.chainB.GetSimApp().ICAHostKeeper.SetParams(s.chainB.GetContext(), params)
			},
			true,
		},
		{
			"interchain account successfully executes govtypes.MsgSubmitProposal",
			func() {
				interchainAccountAddr, found := s.chainB.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(s.chainB.GetContext(), ibctesting.FirstConnectionID, path.EndpointA.ChannelConfig.PortID)
				s.Require().True(found)

				testProposal := &govtypes.TextProposal{
					Title:       "IBC Gov Proposal",
					Description: "tokens for all!",
				}

				protoAny, err := codectypes.NewAnyWithValue(testProposal)
				s.Require().NoError(err)

				msg := &govtypes.MsgSubmitProposal{
					Content:        protoAny,
					InitialDeposit: sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(5000))),
					Proposer:       interchainAccountAddr,
				}

				data, err := icatypes.SerializeCosmosTx(s.chainA.GetSimApp().AppCodec(), []proto.Message{msg})
				s.Require().NoError(err)

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.EXECUTE_TX,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()

				params := types.NewParams(true, []string{sdk.MsgTypeURL(msg)})
				s.chainB.GetSimApp().ICAHostKeeper.SetParams(s.chainB.GetContext(), params)
			},
			true,
		},
		{
			"interchain account successfully executes govtypes.MsgVote",
			func() {
				interchainAccountAddr, found := s.chainB.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(s.chainB.GetContext(), ibctesting.FirstConnectionID, path.EndpointA.ChannelConfig.PortID)
				s.Require().True(found)

				// Populate the gov keeper in advance with an active proposal
				testProposal := &govtypes.TextProposal{
					Title:       "IBC Gov Proposal",
					Description: "tokens for all!",
				}

				proposalMsg, err := govv1.NewLegacyContent(testProposal, interchainAccountAddr)
				s.Require().NoError(err)

				proposal, err := govv1.NewProposal([]sdk.Msg{proposalMsg}, govtypes.DefaultStartingProposalID, s.chainA.GetContext().BlockTime(), s.chainA.GetContext().BlockTime(), "test proposal", "title", "description", sdk.AccAddress(interchainAccountAddr))
				s.Require().NoError(err)

				s.chainB.GetSimApp().GovKeeper.SetProposal(s.chainB.GetContext(), proposal)
				s.chainB.GetSimApp().GovKeeper.ActivateVotingPeriod(s.chainB.GetContext(), proposal)

				msg := &govtypes.MsgVote{
					ProposalId: govtypes.DefaultStartingProposalID,
					Voter:      interchainAccountAddr,
					Option:     govtypes.OptionYes,
				}

				data, err := icatypes.SerializeCosmosTx(s.chainA.GetSimApp().AppCodec(), []proto.Message{msg})
				s.Require().NoError(err)

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.EXECUTE_TX,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()

				params := types.NewParams(true, []string{sdk.MsgTypeURL(msg)})
				s.chainB.GetSimApp().ICAHostKeeper.SetParams(s.chainB.GetContext(), params)
			},
			true,
		},
		{
			"interchain account successfully executes disttypes.MsgFundCommunityPool",
			func() {
				interchainAccountAddr, found := s.chainB.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(s.chainB.GetContext(), ibctesting.FirstConnectionID, path.EndpointA.ChannelConfig.PortID)
				s.Require().True(found)

				msg := &disttypes.MsgFundCommunityPool{
					Amount:    sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(5000))),
					Depositor: interchainAccountAddr,
				}

				data, err := icatypes.SerializeCosmosTx(s.chainA.GetSimApp().AppCodec(), []proto.Message{msg})
				s.Require().NoError(err)

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.EXECUTE_TX,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()

				params := types.NewParams(true, []string{sdk.MsgTypeURL(msg)})
				s.chainB.GetSimApp().ICAHostKeeper.SetParams(s.chainB.GetContext(), params)
			},
			true,
		},
		{
			"interchain account successfully executes disttypes.MsgSetWithdrawAddress",
			func() {
				interchainAccountAddr, found := s.chainB.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(s.chainB.GetContext(), ibctesting.FirstConnectionID, path.EndpointA.ChannelConfig.PortID)
				s.Require().True(found)

				msg := &disttypes.MsgSetWithdrawAddress{
					DelegatorAddress: interchainAccountAddr,
					WithdrawAddress:  s.chainB.SenderAccount.GetAddress().String(),
				}

				data, err := icatypes.SerializeCosmosTx(s.chainA.GetSimApp().AppCodec(), []proto.Message{msg})
				s.Require().NoError(err)

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.EXECUTE_TX,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()

				params := types.NewParams(true, []string{sdk.MsgTypeURL(msg)})
				s.chainB.GetSimApp().ICAHostKeeper.SetParams(s.chainB.GetContext(), params)
			},
			true,
		},
		{
			"interchain account successfully executes transfertypes.MsgTransfer",
			func() {
				transferPath := ibctesting.NewPath(s.chainB, s.chainC)
				transferPath.EndpointA.ChannelConfig.PortID = ibctesting.TransferPort
				transferPath.EndpointB.ChannelConfig.PortID = ibctesting.TransferPort
				transferPath.EndpointA.ChannelConfig.Version = transfertypes.Version
				transferPath.EndpointB.ChannelConfig.Version = transfertypes.Version

				s.coordinator.Setup(transferPath)

				interchainAccountAddr, found := s.chainB.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(s.chainB.GetContext(), ibctesting.FirstConnectionID, path.EndpointA.ChannelConfig.PortID)
				s.Require().True(found)

				msg := &transfertypes.MsgTransfer{
					SourcePort:       transferPath.EndpointA.ChannelConfig.PortID,
					SourceChannel:    transferPath.EndpointA.ChannelID,
					Token:            sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100)),
					Sender:           interchainAccountAddr,
					Receiver:         s.chainA.SenderAccount.GetAddress().String(),
					TimeoutHeight:    clienttypes.NewHeight(1, 100),
					TimeoutTimestamp: uint64(0),
				}

				data, err := icatypes.SerializeCosmosTx(s.chainA.GetSimApp().AppCodec(), []proto.Message{msg})
				s.Require().NoError(err)

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.EXECUTE_TX,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()

				params := types.NewParams(true, []string{sdk.MsgTypeURL(msg)})
				s.chainB.GetSimApp().ICAHostKeeper.SetParams(s.chainB.GetContext(), params)
			},
			true,
		},
		{
			"unregistered sdk.Msg",
			func() {
				msg := &banktypes.MsgSendResponse{}

				data, err := icatypes.SerializeCosmosTx(s.chainA.GetSimApp().AppCodec(), []proto.Message{msg})
				s.Require().NoError(err)

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.EXECUTE_TX,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()

				params := types.NewParams(true, []string{"/" + proto.MessageName(msg)})
				s.chainB.GetSimApp().ICAHostKeeper.SetParams(s.chainB.GetContext(), params)
			},
			false,
		},
		{
			"cannot unmarshal interchain account packet data",
			func() {
				packetData = []byte{}
			},
			false,
		},
		{
			"cannot deserialize interchain account packet data messages",
			func() {
				data := []byte("invalid packet data")

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.EXECUTE_TX,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()
			},
			false,
		},
		{
			"invalid packet type - UNSPECIFIED",
			func() {
				data, err := icatypes.SerializeCosmosTx(s.chainA.GetSimApp().AppCodec(), []proto.Message{&banktypes.MsgSend{}})
				s.Require().NoError(err)

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.UNSPECIFIED,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()
			},
			false,
		},
		{
			"unauthorised: interchain account not found for controller port ID",
			func() {
				path.EndpointA.ChannelConfig.PortID = "invalid-port-id"

				data, err := icatypes.SerializeCosmosTx(s.chainA.GetSimApp().AppCodec(), []proto.Message{&banktypes.MsgSend{}})
				s.Require().NoError(err)

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.EXECUTE_TX,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()
			},
			false,
		},
		{
			"unauthorised: message type not allowed", // NOTE: do not update params to explicitly force the error
			func() {
				msg := &banktypes.MsgSend{
					FromAddress: s.chainB.SenderAccount.GetAddress().String(),
					ToAddress:   s.chainB.SenderAccount.GetAddress().String(),
					Amount:      sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100))),
				}

				data, err := icatypes.SerializeCosmosTx(s.chainA.GetSimApp().AppCodec(), []proto.Message{msg})
				s.Require().NoError(err)

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.EXECUTE_TX,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()
			},
			false,
		},
		{
			"unauthorised: signer address is not the interchain account associated with the controller portID",
			func() {
				msg := &banktypes.MsgSend{
					FromAddress: s.chainB.SenderAccount.GetAddress().String(), // unexpected signer
					ToAddress:   s.chainB.SenderAccount.GetAddress().String(),
					Amount:      sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100))),
				}

				data, err := icatypes.SerializeCosmosTx(s.chainA.GetSimApp().AppCodec(), []proto.Message{msg})
				s.Require().NoError(err)

				icaPacketData := icatypes.InterchainAccountPacketData{
					Type: icatypes.EXECUTE_TX,
					Data: data,
				}

				packetData = icaPacketData.GetBytes()

				params := types.NewParams(true, []string{sdk.MsgTypeURL(msg)})
				s.chainB.GetSimApp().ICAHostKeeper.SetParams(s.chainB.GetContext(), params)
			},
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.msg, func() {
			s.SetupTest() // reset

			path = NewICAPath(s.chainA, s.chainB)
			s.coordinator.SetupConnections(path)

			err := SetupICAPath(path, TestOwnerAddress)
			s.Require().NoError(err)

			portID, err := icatypes.NewControllerPortID(TestOwnerAddress)
			s.Require().NoError(err)

			// Get the address of the interchain account stored in state during handshake step
			storedAddr, found := s.chainB.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(s.chainB.GetContext(), ibctesting.FirstConnectionID, portID)
			s.Require().True(found)

			icaAddr, err := sdk.AccAddressFromBech32(storedAddr)
			s.Require().NoError(err)

			// Check if account is created
			interchainAccount := s.chainB.GetSimApp().AccountKeeper.GetAccount(s.chainB.GetContext(), icaAddr)
			s.Require().Equal(interchainAccount.GetAddress().String(), storedAddr)

			s.fundICAWallet(s.chainB.GetContext(), path.EndpointA.ChannelConfig.PortID, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(10000))))

			tc.malleate() // malleate mutates test data

			packet := channeltypes.NewPacket(
				packetData,
				s.chainA.SenderAccount.GetSequence(),
				path.EndpointA.ChannelConfig.PortID,
				path.EndpointA.ChannelID,
				path.EndpointB.ChannelConfig.PortID,
				path.EndpointB.ChannelID,
				clienttypes.NewHeight(1, 100),
				0,
			)

			txResponse, err := s.chainB.GetSimApp().ICAHostKeeper.OnRecvPacket(s.chainB.GetContext(), packet)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().NotNil(txResponse)
			} else {
				s.Require().Error(err)
				s.Require().Nil(txResponse)
			}
		})
	}
}

func (s *KeeperTestSuite) fundICAWallet(ctx sdk.Context, portID string, amount sdk.Coins) {
	interchainAccountAddr, found := s.chainB.GetSimApp().ICAHostKeeper.GetInterchainAccountAddress(ctx, ibctesting.FirstConnectionID, portID)
	s.Require().True(found)

	msgBankSend := &banktypes.MsgSend{
		FromAddress: s.chainB.SenderAccount.GetAddress().String(),
		ToAddress:   interchainAccountAddr,
		Amount:      amount,
	}

	res, err := s.chainB.SendMsgs(msgBankSend)
	s.Require().NotEmpty(res)
	s.Require().NoError(err)
}
