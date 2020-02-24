package keeper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestUndelegateFromUnbondedValidator(t *testing.T) {
	ctx, _, bk, keeper, _ := CreateTestInput(t, false, 1)
	delTokens := sdk.TokensFromConsensusPower(10)
	delCoins := sdk.NewCoins(sdk.NewCoin(keeper.BondDenom(ctx), delTokens))

	// add bonded tokens to pool for delegations
	notBondedPool := keeper.GetNotBondedPool(ctx)
	oldNotBonded := bk.GetAllBalances(ctx, notBondedPool.GetAddress())
	err := bk.SetBalances(ctx, notBondedPool.GetAddress(), oldNotBonded.Add(delCoins...))
	require.NoError(t, err)
	keeper.supplyKeeper.SetModuleAccount(ctx, notBondedPool)

	// create a validator with a self-delegation
	validator := types.NewValidator(addrVals[0], PKs[0], types.Description{})

	valTokens := sdk.TokensFromConsensusPower(10)
	validator, issuedShares := validator.AddTokensFromDel(valTokens)
	require.Equal(t, valTokens, issuedShares.RoundInt())
	validator = TestingUpdateValidator(keeper, ctx, validator, true)
	require.True(t, validator.IsBonded())

	val0AccAddr := sdk.AccAddress(addrVals[0].Bytes())
	selfDelegation := types.NewDelegation(val0AccAddr, addrVals[0], issuedShares)
	keeper.SetDelegation(ctx, selfDelegation)

	bondedPool := keeper.GetBondedPool(ctx)
	oldBonded := bk.GetAllBalances(ctx, bondedPool.GetAddress())
	err = bk.SetBalances(ctx, bondedPool.GetAddress(), oldBonded.Add(delCoins...))
	require.NoError(t, err)
	keeper.supplyKeeper.SetModuleAccount(ctx, bondedPool)

	// create a second delegation to this validator
	keeper.DeleteValidatorByPowerIndex(ctx, validator)
	validator, issuedShares = validator.AddTokensFromDel(delTokens)
	require.Equal(t, delTokens, issuedShares.RoundInt())
	validator = TestingUpdateValidator(keeper, ctx, validator, true)
	require.True(t, validator.IsBonded())
	delegation := types.NewDelegation(addrDels[0], addrVals[0], issuedShares)
	keeper.SetDelegation(ctx, delegation)

	ctx = ctx.WithBlockHeight(10)
	ctx = ctx.WithBlockTime(time.Unix(333, 0))

	// unbond the all self-delegation to put validator in unbonding state
	_, err = keeper.Undelegate(ctx, val0AccAddr, addrVals[0], valTokens.ToDec())
	require.NoError(t, err)

	// end block
	updates := keeper.ApplyAndReturnValidatorSetUpdates(ctx)
	require.Equal(t, 1, len(updates))

	validator, found := keeper.GetValidator(ctx, addrVals[0])
	require.True(t, found)
	require.Equal(t, ctx.BlockHeight(), validator.UnbondingHeight)
	params := keeper.GetParams(ctx)
	require.True(t, ctx.BlockHeader().Time.Add(params.UnbondingTime).Equal(validator.UnbondingTime))

	// unbond the validator
	ctx = ctx.WithBlockTime(validator.UnbondingTime)
	keeper.UnbondAllMatureValidatorQueue(ctx)

	// Make sure validator is still in state because there is still an outstanding delegation
	validator, found = keeper.GetValidator(ctx, addrVals[0])
	require.True(t, found)
	require.Equal(t, validator.Status, sdk.Unbonded)

	// unbond some of the other delegation's shares
	unbondTokens := sdk.TokensFromConsensusPower(6)
	_, err = keeper.Undelegate(ctx, addrDels[0], addrVals[0], unbondTokens.ToDec())
	require.NoError(t, err)

	// unbond rest of the other delegation's shares
	remainingTokens := delTokens.Sub(unbondTokens)
	_, err = keeper.Undelegate(ctx, addrDels[0], addrVals[0], remainingTokens.ToDec())
	require.NoError(t, err)

	//  now validator should now be deleted from state
	validator, found = keeper.GetValidator(ctx, addrVals[0])
	require.False(t, found, "%v", validator)
}

func TestUnbondingAllDelegationFromValidator(t *testing.T) {
	ctx, _, bk, keeper, _ := CreateTestInput(t, false, 0)
	delTokens := sdk.TokensFromConsensusPower(10)
	delCoins := sdk.NewCoins(sdk.NewCoin(keeper.BondDenom(ctx), delTokens))

	// add bonded tokens to pool for delegations
	notBondedPool := keeper.GetNotBondedPool(ctx)
	oldNotBonded := bk.GetAllBalances(ctx, notBondedPool.GetAddress())
	err := bk.SetBalances(ctx, notBondedPool.GetAddress(), oldNotBonded.Add(delCoins...))
	require.NoError(t, err)
	keeper.supplyKeeper.SetModuleAccount(ctx, notBondedPool)

	//create a validator with a self-delegation
	validator := types.NewValidator(addrVals[0], PKs[0], types.Description{})

	valTokens := sdk.TokensFromConsensusPower(10)
	validator, issuedShares := validator.AddTokensFromDel(valTokens)
	require.Equal(t, valTokens, issuedShares.RoundInt())

	validator = TestingUpdateValidator(keeper, ctx, validator, true)
	require.True(t, validator.IsBonded())
	val0AccAddr := sdk.AccAddress(addrVals[0].Bytes())

	selfDelegation := types.NewDelegation(val0AccAddr, addrVals[0], issuedShares)
	keeper.SetDelegation(ctx, selfDelegation)

	// create a second delegation to this validator
	keeper.DeleteValidatorByPowerIndex(ctx, validator)
	validator, issuedShares = validator.AddTokensFromDel(delTokens)
	require.Equal(t, delTokens, issuedShares.RoundInt())

	bondedPool := keeper.GetBondedPool(ctx)
	oldBonded := bk.GetAllBalances(ctx, bondedPool.GetAddress())
	err = bk.SetBalances(ctx, bondedPool.GetAddress(), oldBonded.Add(delCoins...))
	require.NoError(t, err)
	keeper.supplyKeeper.SetModuleAccount(ctx, bondedPool)

	validator = TestingUpdateValidator(keeper, ctx, validator, true)
	require.True(t, validator.IsBonded())

	delegation := types.NewDelegation(addrDels[0], addrVals[0], issuedShares)
	keeper.SetDelegation(ctx, delegation)

	ctx = ctx.WithBlockHeight(10)
	ctx = ctx.WithBlockTime(time.Unix(333, 0))

	// unbond the all self-delegation to put validator in unbonding state
	_, err = keeper.Undelegate(ctx, val0AccAddr, addrVals[0], valTokens.ToDec())
	require.NoError(t, err)

	// end block
	updates := keeper.ApplyAndReturnValidatorSetUpdates(ctx)
	require.Equal(t, 1, len(updates))

	// unbond all the remaining delegation
	_, err = keeper.Undelegate(ctx, addrDels[0], addrVals[0], delTokens.ToDec())
	require.NoError(t, err)

	// validator should still be in state and still be in unbonding state
	validator, found := keeper.GetValidator(ctx, addrVals[0])
	require.True(t, found)
	require.Equal(t, validator.Status, sdk.Unbonding)

	// unbond the validator
	ctx = ctx.WithBlockTime(validator.UnbondingTime)
	keeper.UnbondAllMatureValidatorQueue(ctx)

	// validator should now be deleted from state
	_, found = keeper.GetValidator(ctx, addrVals[0])
	require.False(t, found)
}

// Make sure that that the retrieving the delegations doesn't affect the state
func TestGetRedelegationsFromSrcValidator(t *testing.T) {
	ctx, _, _, keeper, _ := CreateTestInput(t, false, 0)

	rd := types.NewRedelegation(addrDels[0], addrVals[0], addrVals[1], 0,
		time.Unix(0, 0), sdk.NewInt(5),
		sdk.NewDec(5))

	// set and retrieve a record
	keeper.SetRedelegation(ctx, rd)
	resBond, found := keeper.GetRedelegation(ctx, addrDels[0], addrVals[0], addrVals[1])
	require.True(t, found)

	// get the redelegations one time
	redelegations := keeper.GetRedelegationsFromSrcValidator(ctx, addrVals[0])
	require.Equal(t, 1, len(redelegations))
	require.True(t, redelegations[0].Equal(resBond))

	// get the redelegations a second time, should be exactly the same
	redelegations = keeper.GetRedelegationsFromSrcValidator(ctx, addrVals[0])
	require.Equal(t, 1, len(redelegations))
	require.True(t, redelegations[0].Equal(resBond))
}

// tests Get/Set/Remove/Has UnbondingDelegation
func TestRedelegation(t *testing.T) {
	ctx, _, _, keeper, _ := CreateTestInput(t, false, 0)

	rd := types.NewRedelegation(addrDels[0], addrVals[0], addrVals[1], 0,
		time.Unix(0, 0), sdk.NewInt(5),
		sdk.NewDec(5))

	// test shouldn't have and redelegations
	has := keeper.HasReceivingRedelegation(ctx, addrDels[0], addrVals[1])
	require.False(t, has)

	// set and retrieve a record
	keeper.SetRedelegation(ctx, rd)
	resRed, found := keeper.GetRedelegation(ctx, addrDels[0], addrVals[0], addrVals[1])
	require.True(t, found)

	redelegations := keeper.GetRedelegationsFromSrcValidator(ctx, addrVals[0])
	require.Equal(t, 1, len(redelegations))
	require.True(t, redelegations[0].Equal(resRed))

	redelegations = keeper.GetRedelegations(ctx, addrDels[0], 5)
	require.Equal(t, 1, len(redelegations))
	require.True(t, redelegations[0].Equal(resRed))

	redelegations = keeper.GetAllRedelegations(ctx, addrDels[0], nil, nil)
	require.Equal(t, 1, len(redelegations))
	require.True(t, redelegations[0].Equal(resRed))

	// check if has the redelegation
	has = keeper.HasReceivingRedelegation(ctx, addrDels[0], addrVals[1])
	require.True(t, has)

	// modify a records, save, and retrieve
	rd.Entries[0].SharesDst = sdk.NewDec(21)
	keeper.SetRedelegation(ctx, rd)

	resRed, found = keeper.GetRedelegation(ctx, addrDels[0], addrVals[0], addrVals[1])
	require.True(t, found)
	require.True(t, rd.Equal(resRed))

	redelegations = keeper.GetRedelegationsFromSrcValidator(ctx, addrVals[0])
	require.Equal(t, 1, len(redelegations))
	require.True(t, redelegations[0].Equal(resRed))

	redelegations = keeper.GetRedelegations(ctx, addrDels[0], 5)
	require.Equal(t, 1, len(redelegations))
	require.True(t, redelegations[0].Equal(resRed))

	// delete a record
	keeper.RemoveRedelegation(ctx, rd)
	_, found = keeper.GetRedelegation(ctx, addrDels[0], addrVals[0], addrVals[1])
	require.False(t, found)

	redelegations = keeper.GetRedelegations(ctx, addrDels[0], 5)
	require.Equal(t, 0, len(redelegations))

	redelegations = keeper.GetAllRedelegations(ctx, addrDels[0], nil, nil)
	require.Equal(t, 0, len(redelegations))
}

func TestRedelegateToSameValidator(t *testing.T) {
	ctx, _, bk, keeper, _ := CreateTestInput(t, false, 0)
	valTokens := sdk.TokensFromConsensusPower(10)
	startCoins := sdk.NewCoins(sdk.NewCoin(keeper.BondDenom(ctx), valTokens))

	// add bonded tokens to pool for delegations
	notBondedPool := keeper.GetNotBondedPool(ctx)
	oldNotBonded := bk.GetAllBalances(ctx, notBondedPool.GetAddress())
	err := bk.SetBalances(ctx, notBondedPool.GetAddress(), oldNotBonded.Add(startCoins...))
	require.NoError(t, err)
	keeper.supplyKeeper.SetModuleAccount(ctx, notBondedPool)

	// create a validator with a self-delegation
	validator := types.NewValidator(addrVals[0], PKs[0], types.Description{})
	validator, issuedShares := validator.AddTokensFromDel(valTokens)
	require.Equal(t, valTokens, issuedShares.RoundInt())
	validator = TestingUpdateValidator(keeper, ctx, validator, true)
	require.True(t, validator.IsBonded())

	val0AccAddr := sdk.AccAddress(addrVals[0].Bytes())
	selfDelegation := types.NewDelegation(val0AccAddr, addrVals[0], issuedShares)
	keeper.SetDelegation(ctx, selfDelegation)

	_, err = keeper.BeginRedelegation(ctx, val0AccAddr, addrVals[0], addrVals[0], sdk.NewDec(5))
	require.Error(t, err)
}

func TestRedelegationMaxEntries(t *testing.T) {
	ctx, _, bk, keeper, _ := CreateTestInput(t, false, 0)
	startTokens := sdk.TokensFromConsensusPower(20)
	startCoins := sdk.NewCoins(sdk.NewCoin(keeper.BondDenom(ctx), startTokens))

	// add bonded tokens to pool for delegations
	notBondedPool := keeper.GetNotBondedPool(ctx)
	oldNotBonded := bk.GetAllBalances(ctx, notBondedPool.GetAddress())
	err := bk.SetBalances(ctx, notBondedPool.GetAddress(), oldNotBonded.Add(startCoins...))
	require.NoError(t, err)
	keeper.supplyKeeper.SetModuleAccount(ctx, notBondedPool)

	// create a validator with a self-delegation
	validator := types.NewValidator(addrVals[0], PKs[0], types.Description{})
	valTokens := sdk.TokensFromConsensusPower(10)
	validator, issuedShares := validator.AddTokensFromDel(valTokens)
	require.Equal(t, valTokens, issuedShares.RoundInt())
	validator = TestingUpdateValidator(keeper, ctx, validator, true)
	val0AccAddr := sdk.AccAddress(addrVals[0].Bytes())
	selfDelegation := types.NewDelegation(val0AccAddr, addrVals[0], issuedShares)
	keeper.SetDelegation(ctx, selfDelegation)

	// create a second validator
	validator2 := types.NewValidator(addrVals[1], PKs[1], types.Description{})
	validator2, issuedShares = validator2.AddTokensFromDel(valTokens)
	require.Equal(t, valTokens, issuedShares.RoundInt())

	validator2 = TestingUpdateValidator(keeper, ctx, validator2, true)
	require.Equal(t, sdk.Bonded, validator2.Status)

	maxEntries := keeper.MaxEntries(ctx)

	// redelegations should pass
	var completionTime time.Time
	for i := uint32(0); i < maxEntries; i++ {
		var err error
		completionTime, err = keeper.BeginRedelegation(ctx, val0AccAddr, addrVals[0], addrVals[1], sdk.NewDec(1))
		require.NoError(t, err)
	}

	// an additional redelegation should fail due to max entries
	_, err = keeper.BeginRedelegation(ctx, val0AccAddr, addrVals[0], addrVals[1], sdk.NewDec(1))
	require.Error(t, err)

	// mature redelegations
	ctx = ctx.WithBlockTime(completionTime)
	err = keeper.CompleteRedelegation(ctx, val0AccAddr, addrVals[0], addrVals[1])
	require.NoError(t, err)

	// redelegation should work again
	_, err = keeper.BeginRedelegation(ctx, val0AccAddr, addrVals[0], addrVals[1], sdk.NewDec(1))
	require.NoError(t, err)
}

func TestRedelegateSelfDelegation(t *testing.T) {
	ctx, _, bk, keeper, _ := CreateTestInput(t, false, 0)
	startTokens := sdk.TokensFromConsensusPower(30)
	startCoins := sdk.NewCoins(sdk.NewCoin(keeper.BondDenom(ctx), startTokens))

	// add bonded tokens to pool for delegations
	notBondedPool := keeper.GetNotBondedPool(ctx)
	oldNotBonded := bk.GetAllBalances(ctx, notBondedPool.GetAddress())
	err := bk.SetBalances(ctx, notBondedPool.GetAddress(), oldNotBonded.Add(startCoins...))
	require.NoError(t, err)
	keeper.supplyKeeper.SetModuleAccount(ctx, notBondedPool)

	//create a validator with a self-delegation
	validator := types.NewValidator(addrVals[0], PKs[0], types.Description{})
	valTokens := sdk.TokensFromConsensusPower(10)
	validator, issuedShares := validator.AddTokensFromDel(valTokens)
	require.Equal(t, valTokens, issuedShares.RoundInt())

	validator = TestingUpdateValidator(keeper, ctx, validator, true)

	val0AccAddr := sdk.AccAddress(addrVals[0].Bytes())
	selfDelegation := types.NewDelegation(val0AccAddr, addrVals[0], issuedShares)
	keeper.SetDelegation(ctx, selfDelegation)

	// create a second validator
	validator2 := types.NewValidator(addrVals[1], PKs[1], types.Description{})
	validator2, issuedShares = validator2.AddTokensFromDel(valTokens)
	require.Equal(t, valTokens, issuedShares.RoundInt())
	validator2 = TestingUpdateValidator(keeper, ctx, validator2, true)
	require.Equal(t, sdk.Bonded, validator2.Status)

	// create a second delegation to validator 1
	delTokens := sdk.TokensFromConsensusPower(10)
	validator, issuedShares = validator.AddTokensFromDel(delTokens)
	require.Equal(t, delTokens, issuedShares.RoundInt())
	validator = TestingUpdateValidator(keeper, ctx, validator, true)

	delegation := types.NewDelegation(addrDels[0], addrVals[0], issuedShares)
	keeper.SetDelegation(ctx, delegation)

	_, err = keeper.BeginRedelegation(ctx, val0AccAddr, addrVals[0], addrVals[1], delTokens.ToDec())
	require.NoError(t, err)

	// end block
	updates := keeper.ApplyAndReturnValidatorSetUpdates(ctx)
	require.Equal(t, 2, len(updates))

	validator, found := keeper.GetValidator(ctx, addrVals[0])
	require.True(t, found)
	require.Equal(t, valTokens, validator.Tokens)
	require.Equal(t, sdk.Unbonding, validator.Status)
}

func TestRedelegateFromUnbondingValidator(t *testing.T) {
	ctx, _, bk, keeper, _ := CreateTestInput(t, false, 0)
	startTokens := sdk.TokensFromConsensusPower(30)
	startCoins := sdk.NewCoins(sdk.NewCoin(keeper.BondDenom(ctx), startTokens))

	// add bonded tokens to pool for delegations
	notBondedPool := keeper.GetNotBondedPool(ctx)
	oldNotBonded := bk.GetAllBalances(ctx, notBondedPool.GetAddress())
	err := bk.SetBalances(ctx, notBondedPool.GetAddress(), oldNotBonded.Add(startCoins...))
	require.NoError(t, err)
	keeper.supplyKeeper.SetModuleAccount(ctx, notBondedPool)

	//create a validator with a self-delegation
	validator := types.NewValidator(addrVals[0], PKs[0], types.Description{})

	valTokens := sdk.TokensFromConsensusPower(10)
	validator, issuedShares := validator.AddTokensFromDel(valTokens)
	require.Equal(t, valTokens, issuedShares.RoundInt())
	validator = TestingUpdateValidator(keeper, ctx, validator, true)
	val0AccAddr := sdk.AccAddress(addrVals[0].Bytes())
	selfDelegation := types.NewDelegation(val0AccAddr, addrVals[0], issuedShares)
	keeper.SetDelegation(ctx, selfDelegation)

	// create a second delegation to this validator
	keeper.DeleteValidatorByPowerIndex(ctx, validator)
	delTokens := sdk.TokensFromConsensusPower(10)
	validator, issuedShares = validator.AddTokensFromDel(delTokens)
	require.Equal(t, delTokens, issuedShares.RoundInt())
	validator = TestingUpdateValidator(keeper, ctx, validator, true)
	delegation := types.NewDelegation(addrDels[0], addrVals[0], issuedShares)
	keeper.SetDelegation(ctx, delegation)

	// create a second validator
	validator2 := types.NewValidator(addrVals[1], PKs[1], types.Description{})
	validator2, issuedShares = validator2.AddTokensFromDel(valTokens)
	require.Equal(t, valTokens, issuedShares.RoundInt())
	validator2 = TestingUpdateValidator(keeper, ctx, validator2, true)

	header := ctx.BlockHeader()
	blockHeight := int64(10)
	header.Height = blockHeight
	blockTime := time.Unix(333, 0)
	header.Time = blockTime
	ctx = ctx.WithBlockHeader(header)

	// unbond the all self-delegation to put validator in unbonding state
	_, err = keeper.Undelegate(ctx, val0AccAddr, addrVals[0], delTokens.ToDec())
	require.NoError(t, err)

	// end block
	updates := keeper.ApplyAndReturnValidatorSetUpdates(ctx)
	require.Equal(t, 1, len(updates))

	validator, found := keeper.GetValidator(ctx, addrVals[0])
	require.True(t, found)
	require.Equal(t, blockHeight, validator.UnbondingHeight)
	params := keeper.GetParams(ctx)
	require.True(t, blockTime.Add(params.UnbondingTime).Equal(validator.UnbondingTime))

	//change the context
	header = ctx.BlockHeader()
	blockHeight2 := int64(20)
	header.Height = blockHeight2
	blockTime2 := time.Unix(444, 0)
	header.Time = blockTime2
	ctx = ctx.WithBlockHeader(header)

	// unbond some of the other delegation's shares
	redelegateTokens := sdk.TokensFromConsensusPower(6)
	_, err = keeper.BeginRedelegation(ctx, addrDels[0], addrVals[0], addrVals[1], redelegateTokens.ToDec())
	require.NoError(t, err)

	// retrieve the unbonding delegation
	ubd, found := keeper.GetRedelegation(ctx, addrDels[0], addrVals[0], addrVals[1])
	require.True(t, found)
	require.Len(t, ubd.Entries, 1)
	assert.Equal(t, blockHeight, ubd.Entries[0].CreationHeight)
	assert.True(t, blockTime.Add(params.UnbondingTime).Equal(ubd.Entries[0].CompletionTime))
}

func TestRedelegateFromUnbondedValidator(t *testing.T) {
	ctx, _, bk, keeper, _ := CreateTestInput(t, false, 0)
	startTokens := sdk.TokensFromConsensusPower(30)
	startCoins := sdk.NewCoins(sdk.NewCoin(keeper.BondDenom(ctx), startTokens))

	// add bonded tokens to pool for delegations
	notBondedPool := keeper.GetNotBondedPool(ctx)
	oldNotBonded := bk.GetAllBalances(ctx, notBondedPool.GetAddress())
	err := bk.SetBalances(ctx, notBondedPool.GetAddress(), oldNotBonded.Add(startCoins...))
	require.NoError(t, err)
	keeper.supplyKeeper.SetModuleAccount(ctx, notBondedPool)

	//create a validator with a self-delegation
	validator := types.NewValidator(addrVals[0], PKs[0], types.Description{})

	valTokens := sdk.TokensFromConsensusPower(10)
	validator, issuedShares := validator.AddTokensFromDel(valTokens)
	require.Equal(t, valTokens, issuedShares.RoundInt())
	validator = TestingUpdateValidator(keeper, ctx, validator, true)
	val0AccAddr := sdk.AccAddress(addrVals[0].Bytes())
	selfDelegation := types.NewDelegation(val0AccAddr, addrVals[0], issuedShares)
	keeper.SetDelegation(ctx, selfDelegation)

	// create a second delegation to this validator
	keeper.DeleteValidatorByPowerIndex(ctx, validator)
	delTokens := sdk.TokensFromConsensusPower(10)
	validator, issuedShares = validator.AddTokensFromDel(delTokens)
	require.Equal(t, delTokens, issuedShares.RoundInt())
	validator = TestingUpdateValidator(keeper, ctx, validator, true)
	delegation := types.NewDelegation(addrDels[0], addrVals[0], issuedShares)
	keeper.SetDelegation(ctx, delegation)

	// create a second validator
	validator2 := types.NewValidator(addrVals[1], PKs[1], types.Description{})
	validator2, issuedShares = validator2.AddTokensFromDel(valTokens)
	require.Equal(t, valTokens, issuedShares.RoundInt())
	validator2 = TestingUpdateValidator(keeper, ctx, validator2, true)
	require.Equal(t, sdk.Bonded, validator2.Status)

	ctx = ctx.WithBlockHeight(10)
	ctx = ctx.WithBlockTime(time.Unix(333, 0))

	// unbond the all self-delegation to put validator in unbonding state
	_, err = keeper.Undelegate(ctx, val0AccAddr, addrVals[0], delTokens.ToDec())
	require.NoError(t, err)

	// end block
	updates := keeper.ApplyAndReturnValidatorSetUpdates(ctx)
	require.Equal(t, 1, len(updates))

	validator, found := keeper.GetValidator(ctx, addrVals[0])
	require.True(t, found)
	require.Equal(t, ctx.BlockHeight(), validator.UnbondingHeight)
	params := keeper.GetParams(ctx)
	require.True(t, ctx.BlockHeader().Time.Add(params.UnbondingTime).Equal(validator.UnbondingTime))

	// unbond the validator
	keeper.unbondingToUnbonded(ctx, validator)

	// redelegate some of the delegation's shares
	redelegationTokens := sdk.TokensFromConsensusPower(6)
	_, err = keeper.BeginRedelegation(ctx, addrDels[0], addrVals[0], addrVals[1], redelegationTokens.ToDec())
	require.NoError(t, err)

	// no red should have been found
	red, found := keeper.GetRedelegation(ctx, addrDels[0], addrVals[0], addrVals[1])
	require.False(t, found, "%v", red)
}
