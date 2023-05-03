package keeper

import (
	"reflect"

	errorsmod "cosmossdk.io/errors"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/ibc-go/v7/internal/collections"
	clienttypes "github.com/cosmos/ibc-go/v7/modules/core/02-client/types"
	connectiontypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	"github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
)

// ChanUpgradeInit is called by a module to initiate a channel upgrade handshake with
// a module on another chain.
func (k Keeper) ChanUpgradeInit(
	ctx sdk.Context,
	portID string,
	channelID string,
	upgradeFields types.UpgradeFields,
	upgradeTimeout types.UpgradeTimeout,
) (types.Upgrade, error) {
	channel, found := k.GetChannel(ctx, portID, channelID)
	if !found {
		return types.Upgrade{}, errorsmod.Wrapf(types.ErrChannelNotFound, "port ID (%s) channel ID (%s)", portID, channelID)
	}

	if channel.State != types.OPEN {
		return types.Upgrade{}, errorsmod.Wrapf(types.ErrInvalidChannelState, "expected %s, got %s", types.OPEN, channel.State)
	}

	if err := k.ValidateUpgradeFields(ctx, upgradeFields, channel); err != nil {
		return types.Upgrade{}, err
	}

	proposedUpgrade, err := k.constructProposedUpgrade(ctx, portID, channelID, upgradeFields, upgradeTimeout)
	if err != nil {
		return types.Upgrade{}, errorsmod.Wrap(err, "failed to construct proposed upgrade")
	}

	channel.UpgradeSequence++
	k.SetChannel(ctx, portID, channelID, channel)

	return proposedUpgrade, nil
}

// WriteUpgradeInitChannel writes a channel which has successfully passed the UpgradeInit handshake step.
// An event is emitted for the handshake step.
func (k Keeper) WriteUpgradeInitChannel(ctx sdk.Context, portID, channelID string, currentChannel types.Channel, upgrade types.Upgrade) {
	defer telemetry.IncrCounter(1, "ibc", "channel", "upgrade-init")

	currentChannel.State = types.INITUPGRADE

	k.SetChannel(ctx, portID, channelID, currentChannel)
	k.SetUpgrade(ctx, portID, channelID, upgrade)

	k.Logger(ctx).Info("channel state updated", "port-id", portID, "channel-id", channelID, "previous-state", types.OPEN.String(), "new-state", types.INITUPGRADE.String())

	emitChannelUpgradeInitEvent(ctx, portID, channelID, currentChannel, upgrade)
}

// constructProposedUpgrade returns the proposed upgrade from the provided arguments.
func (k Keeper) constructProposedUpgrade(ctx sdk.Context, portID, channelID string, fields types.UpgradeFields, timeout types.UpgradeTimeout) (types.Upgrade, error) {
	seq, found := k.GetNextSequenceSend(ctx, portID, channelID)
	if !found {
		return types.Upgrade{}, types.ErrSequenceSendNotFound
	}
	return types.Upgrade{
		Fields:             fields,
		Timeout:            timeout,
		LatestSequenceSend: seq - 1,
	}, nil
}

// ChanUpgradeTry is called by a module to accept the first step of a channel upgrade handshake initiated by
// a module on another chain. If this function is successful, the proposed upgrade will be returned. If the upgrade fails, the upgrade sequence will still be incremented but an error will be returned.
func (k Keeper) ChanUpgradeTry(
	ctx sdk.Context,
	portID,
	channelID string,
	proposedConnectionHops []string,
	proposedUpgradeTimeout types.UpgradeTimeout,
	counterpartyProposedUpgrade types.Upgrade,
	counterpartyUpgradeSequence uint64,
	proofCounterpartyChannel,
	proofUpgrade []byte,
	proofHeight clienttypes.Height,
) (types.Upgrade, error) {
	channel, found := k.GetChannel(ctx, portID, channelID)
	if !found {
		return types.Upgrade{}, errorsmod.Wrapf(types.ErrChannelNotFound, "port ID (%s) channel ID (%s)", portID, channelID)
	}

	// the channel state must be in OPEN or INITUPGRADE if we are in a crossing hellos situation
	if !collections.Contains(channel.State, []types.State{types.OPEN, types.INITUPGRADE}) {
		return types.Upgrade{}, errorsmod.Wrapf(types.ErrInvalidChannelState, "expected one of [%s, %s], got %s", types.OPEN, types.INITUPGRADE, channel.State)
	}

	upgradeFields := types.NewUpgradeFields(
		counterpartyProposedUpgrade.Fields.Ordering,
		proposedConnectionHops,
		counterpartyProposedUpgrade.Fields.Version,
	)

	// verify that the timeout set in UpgradeInit has not passed on this chain
	if hasPassed, err := counterpartyProposedUpgrade.Timeout.HasPassed(ctx); hasPassed {
		// abort here and let counterparty timeout the upgrade
		return types.Upgrade{}, errorsmod.Wrap(err, "upgrade timeout has passed: ")
	}

	// if OPEN, then initialize handshake with upgradeFields
	// this should validate the upgrade fields, set the upgrade path and set the final correct sequence.
	var proposedUpgrade types.Upgrade
	var err error
	if channel.State == types.OPEN {
		if err := k.ValidateUpgradeFields(ctx, upgradeFields, channel); err != nil {
			return types.Upgrade{}, err
		}

		proposedUpgrade, err = k.constructProposedUpgrade(ctx, portID, channelID, upgradeFields, proposedUpgradeTimeout)
		if err != nil {
			return types.Upgrade{}, errorsmod.Wrap(err, "failed to construct proposed upgrade")
		}

		if counterpartyUpgradeSequence > channel.UpgradeSequence {
			channel.UpgradeSequence = counterpartyUpgradeSequence
		}

		channel.UpgradeSequence++

		k.SetChannel(ctx, portID, channelID, channel)
		k.SetUpgrade(ctx, portID, channelID, proposedUpgrade)

		// otherwise, assert that the upgrade fields are the same for crossing-hellos case
	} else if channel.State == types.INITUPGRADE {
		currentUpgrade, found := k.GetUpgrade(ctx, portID, channelID)
		if !found {
			return types.Upgrade{}, errorsmod.Wrap(types.ErrInvalidUpgrade, "failed to retrieve upgrade")
		}

		if !reflect.DeepEqual(currentUpgrade.Fields, upgradeFields) {
			return types.Upgrade{}, errorsmod.Wrap(types.ErrInvalidUpgrade, "proposed upgrade fields have changed since UpgradeInit")
		}

		proposedUpgrade = currentUpgrade
	}

	// if the counterparty sequence is not equal to the current sequence, then either the counterparty chain is out-of-sync or
	// the message is out-of-sync and we write an error receipt with our own sequence so that the counterparty can update
	// their sequence as well.
	// We must then increment our sequence so both sides start the next upgrade with a fresh sequence.
	if counterpartyUpgradeSequence != channel.UpgradeSequence {
		errorReceipt := types.NewErrorReceipt(channel.UpgradeSequence, errorsmod.Wrapf(types.ErrInvalidUpgrade, "counterparty chain upgrade sequence <= upgrade sequence (%d <= %d)", counterpartyUpgradeSequence, channel.UpgradeSequence))
		channel.UpgradeSequence++
		// TODO: emit error receipt events
		k.SetUpgradeErrorReceipt(ctx, portID, channelID, errorReceipt)
	}

	connectionEnd, err := k.GetConnection(ctx, channel.ConnectionHops[0])
	if err != nil {
		return types.Upgrade{}, errorsmod.Wrapf(connectiontypes.ErrConnectionNotFound, "failed to retrieve connection on %s. Error: %s", channel.ConnectionHops[0], err.Error())
	}

	// make sure connection is OPEN
	// TODO: how to test this?
	if connectionEnd.GetState() != int32(connectiontypes.OPEN) {
		return types.Upgrade{}, errorsmod.Wrapf(
			connectiontypes.ErrInvalidConnectionState,
			"connection state is not OPEN (got %s)", connectiontypes.State(connectionEnd.GetState()).String(),
		)
	}

	// check the connection associated with the passed in connection hops by the chain sending the TryUpgrade
	// matches the connection associated with the counterparty proposed upgrade
	counterpartyHops := counterpartyProposedUpgrade.Fields.ConnectionHops
	if !reflect.DeepEqual(counterpartyHops, proposedConnectionHops) {
		return types.Upgrade{}, errorsmod.Wrapf(types.ErrInvalidConnectionHops, "proposed connection hops (%s) does not match counterparty proposed connection hops (%s)", proposedConnectionHops, counterpartyHops)
	}

	return proposedUpgrade, nil

}

// WriteUpgradeTryChannel writes a channel which has successfully passed the UpgradeTry handshake step.
// An event is emitted for the handshake step.
func (k Keeper) WriteUpgradeTryChannel(
	ctx sdk.Context,
	portID, channelID string,
	proposedUpgrade types.Upgrade,
) {
	defer telemetry.IncrCounter(1, "ibc", "channel", "upgrade-try")

	currentChannel, found := k.GetChannel(ctx, portID, channelID)
	if !found {
		panic("channel not found")
	}

	previousState := currentChannel.State

	currentChannel.State = types.TRYUPGRADE
	k.SetChannel(ctx, portID, channelID, currentChannel)
	k.SetUpgrade(ctx, portID, channelID, proposedUpgrade)

	// TODO: previous state will not be OPEN in the case of crossing hellos. Determine this state correctly.
	k.Logger(ctx).Info("channel state updated", "port-id", portID, "channel-id", channelID, "previous-state", previousState, "new-state", types.TRYUPGRADE.String())
	emitChannelUpgradeTryEvent(ctx, portID, channelID, currentChannel, proposedUpgrade)
}

// StartFlushUpgradeHandshake updates channel state from INITUPGRADE to TRYUPGRADE
// packet flushing logic should be performed here
// upgrade is blocked on this channelEnd from progressing until flush completes on both ends
func (k Keeper) StartFlushUpgradeHandshake(
	ctx sdk.Context,
	portID, channelID string,
	upgradeFields types.UpgradeFields,
	counterpartyChannel types.Channel,
	counterpartyUpgrade types.Upgrade,
	proofChannel, proofUpgrade []byte,
	proofHeight clienttypes.Height,
) error {
	// TODO
	return nil
}
