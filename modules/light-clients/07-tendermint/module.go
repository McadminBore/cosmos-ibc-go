package tendermint

import (
	"encoding/json"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

var _ module.AppModuleBasic = (*AppModuleBasic)(nil)

// AppModuleBasic defines the basic application module used by the tendermint light client.
// Only the RegisterInterfaces function needs to be implemented. All other function perform
// a no-op.
type AppModuleBasic struct{}

// Name returns the tendermint module name.
func (AppModuleBasic) Name() string {
	return ModuleName
}

// RegisterLegacyAminoCodec performs a no-op. The Tendermint client does not support amino.
func (AppModuleBasic) RegisterLegacyAminoCodec(*codec.LegacyAmino) {}

// RegisterInterfaces registers module concrete types into protobuf Any. This allows core IBC
// to unmarshal tendermint light client types.
func (AppModuleBasic) RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	RegisterInterfaces(registry)
}

// DefaultGenesis performs a no-op. Genesis is not supported for the tendermint light client.
func (AppModuleBasic) DefaultGenesis(_ codec.JSONCodec) json.RawMessage {
	return nil
}

// ValidateGenesis performs a no-op. Genesis is not supported for the tendermint light cilent.
func (AppModuleBasic) ValidateGenesis(_ codec.JSONCodec, _ client.TxEncodingConfig, _ json.RawMessage) error {
	return nil
}

// RegisterGRPCGatewayRoutes performs a no-op.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(_ client.Context, _ *runtime.ServeMux) {}

// GetTxCmd performs a no-op. Please see the 02-client cli commands.
func (AppModuleBasic) GetTxCmd() *cobra.Command {
	return nil
}

// GetQueryCmd performs a no-op. Please see the 02-client cli commands.
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return nil
}
