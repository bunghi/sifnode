package keeper

import (
	"errors"
	"fmt"

	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Sifchain/sifnode/x/ethbridge/types"
	"github.com/Sifchain/sifnode/x/oracle"
)

// Keeper maintains the link to data storage and
// exposes getter/setter methods for the various parts of the state machine
type Keeper struct {
	cdc *codec.Codec // The wire codec for binary encoding/decoding.

	supplyKeeper types.SupplyKeeper
	oracleKeeper types.OracleKeeper
	storeKey     sdk.StoreKey
}

// NewKeeper creates new instances of the oracle Keeper
func NewKeeper(cdc *codec.Codec, supplyKeeper types.SupplyKeeper, oracleKeeper types.OracleKeeper, storeKey sdk.StoreKey) Keeper {
	return Keeper{
		cdc:          cdc,
		supplyKeeper: supplyKeeper,
		oracleKeeper: oracleKeeper,
		storeKey:     storeKey,
	}
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// ProcessClaim processes a new claim coming in from a validator
func (k Keeper) ProcessClaim(ctx sdk.Context, claim types.EthBridgeClaim) (oracle.Status, error) {
	oracleClaim, err := types.CreateOracleClaimFromEthClaim(k.cdc, claim)
	if err != nil {
		return oracle.Status{}, err
	}

	return k.oracleKeeper.ProcessClaim(ctx, oracleClaim)
}

// ProcessSuccessfulClaim processes a claim that has just completed successfully with consensus
func (k Keeper) ProcessSuccessfulClaim(ctx sdk.Context, claim string) error {
	oracleClaim, err := types.CreateOracleClaimFromOracleString(claim)
	if err != nil {
		return err
	}

	receiverAddress := oracleClaim.CosmosReceiver

	var coins sdk.Coins
	switch oracleClaim.ClaimType {
	case types.LockText:
		symbol := fmt.Sprintf("%v%v", types.PeggedCoinPrefix, oracleClaim.Symbol)
		k.AddPeggyToken(ctx, symbol)

		coins = sdk.Coins{sdk.NewCoin(symbol, oracleClaim.Amount)}
		err = k.supplyKeeper.MintCoins(ctx, types.ModuleName, coins)
	case types.BurnText:
		coins = sdk.Coins{sdk.NewCoin(oracleClaim.Symbol, oracleClaim.Amount)}
		err = k.supplyKeeper.MintCoins(ctx, types.ModuleName, coins)
	default:
		err = types.ErrInvalidClaimType
	}

	if err != nil {
		return err
	}

	if err := k.supplyKeeper.SendCoinsFromModuleToAccount(
		ctx, types.ModuleName, receiverAddress, coins,
	); err != nil {
		panic(err)
	}

	return nil
}

// ProcessBurn processes the burn of bridged coins from the given sender
func (k Keeper) ProcessBurn(ctx sdk.Context, cosmosSender sdk.AccAddress, cosmosSenderSequence uint64, amount sdk.Coins) error {
	err := k.InsertNewID(ctx, BuildLockBurnID(cosmosSender, cosmosSenderSequence))
	if err != nil {
		return err
	}

	if err := k.supplyKeeper.SendCoinsFromAccountToModule(
		ctx, cosmosSender, types.ModuleName, amount,
	); err != nil {
		return err
	}

	if err := k.supplyKeeper.BurnCoins(ctx, types.ModuleName, amount); err != nil {
		panic(err)
	}

	return nil
}

// ProcessUnburn processes the revert burn of bridged coins from the given sender
func (k Keeper) ProcessUnburn(ctx sdk.Context, cosmosSender sdk.AccAddress, cosmosSenderSequence uint64, amount sdk.Coins, validatorAddress sdk.ValAddress) error {
	if !k.oracleKeeper.ValidateAddress(ctx, validatorAddress) {
		return errors.New("validator not in the white list")
	}

	updated, err := k.SetLockBurnID(ctx, BuildLockBurnID(cosmosSender, cosmosSenderSequence))
	if err != nil {
		return err
	}

	if !updated {
		return nil
	}

	if err := k.supplyKeeper.MintCoins(ctx, types.ModuleName, amount); err != nil {
		return err
	}

	if err := k.supplyKeeper.SendCoinsFromModuleToAccount(
		ctx, types.ModuleName, cosmosSender, amount,
	); err != nil {
		panic(err)
	}

	return nil
}

// ProcessLock processes the lockup of cosmos coins from the given sender
func (k Keeper) ProcessLock(ctx sdk.Context, cosmosSender sdk.AccAddress, cosmosSenderSequence uint64, amount sdk.Coins) error {
	err := k.InsertNewID(ctx, BuildLockBurnID(cosmosSender, cosmosSenderSequence))
	if err != nil {
		return err
	}
	return k.supplyKeeper.SendCoinsFromAccountToModule(ctx, cosmosSender, types.ModuleName, amount)
}

// ProcessUnlock processes the revert lockup of cosmos coins from the given sender
func (k Keeper) ProcessUnlock(ctx sdk.Context, cosmosSender sdk.AccAddress, cosmosSenderSequence uint64, amount sdk.Coins, validatorAddress sdk.ValAddress) error {
	if !k.oracleKeeper.ValidateAddress(ctx, validatorAddress) {
		return errors.New("validator not in the white list")
	}
	updated, err := k.SetLockBurnID(ctx, BuildLockBurnID(cosmosSender, cosmosSenderSequence))
	if err != nil {
		return err
	}

	if !updated {
		return nil
	}

	return k.supplyKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, cosmosSender, amount)
}

// ProcessUpdateWhiteListValidator processes the update whitelist validator from admin
func (k Keeper) ProcessUpdateWhiteListValidator(ctx sdk.Context, cosmosSender sdk.AccAddress, validator sdk.ValAddress, operationtype string) error {
	return k.oracleKeeper.ProcessUpdateWhiteListValidator(ctx, cosmosSender, validator, operationtype)
}

// Exists chec if the key existed in db.
func (k Keeper) Exists(ctx sdk.Context, key []byte) bool {
	store := ctx.KVStore(k.storeKey)
	return store.Has(key)
}
