package keeper

import (
	"github.com/comdex-official/comdex/x/liquidation/types"
	vaulttypes "github.com/comdex-official/comdex/x/vault/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	protobuftypes "github.com/gogo/protobuf/types"
)

func (k Keeper) LiquidateVaults(ctx sdk.Context) error {
	appIds := k.GetAppIds(ctx).WhitelistedAppMappingIds
	for i := range appIds {
		vaultsMap, _ := k.GetAppExtendedPairVaultMapping(ctx, appIds[i])

		vaults := vaultsMap.ExtendedPairVaults
		for j := range vaults {
			vaultIds := vaults[j].VaultIds
			for l := range vaultIds {
				vault, _ := k.GetVault(ctx, vaultIds[l])

				extPair, _ := k.GetPairsVault(ctx, vault.ExtendedPairVaultID)
				/*pair, _ := k.GetPair(ctx, extPair.PairId)

				assetIn, found := k.GetAsset(ctx, pair.AssetIn)
				if !found {
					continue
				}

				assetOut, found := k.GetAsset(ctx, pair.AssetOut)
				if !found {
					continue
				}*/
				liqRatio := extPair.LiquidationRatio
				collateralitzationRatio, err := k.CalculateCollaterlizationRatio(ctx, vault.ExtendedPairVaultID, vault.AmountIn, vault.AmountOut)
				if err != nil {
					continue
				}

				if sdk.Dec.LT(collateralitzationRatio, liqRatio) {
					err := k.CreateLockedVault(ctx, vault, collateralitzationRatio, appIds[i])
					if err != nil {
						continue
					}
					k.DeleteVault(ctx, vault.Id)
					//k.UpdateUserVaultIdMapping(ctx, vault.Owner, vault.Id, false)
					//k.UpdateCollateralVaultIdMapping(ctx, assetIn.Denom, assetOut.Denom, vault.Id, false)
				}
			}
		}
	}
	return nil
}

func (k Keeper) CreateLockedVault(ctx sdk.Context, vault vaulttypes.Vault, collateralizationRatio sdk.Dec, appId uint64) error {

	k.GetLockedVaultIDbyApp(ctx, appId)

	/*var value = types.LockedVault{
		LockedVaultId:                lockedVaultId,
		AppVaultTypeId:               string(appId),
		OriginalVaultId:              (vault.Id),
		PairId:                       vault.PairID,
		Owner:                        vault.Owner,
		AmountIn:                     vault.AmountIn,
		AmountOut:                    vault.AmountOut,
		Initiator:                    types.ModuleName,
		IsAuctionComplete:            false,
		IsAuctionInProgress:          false,
		CrAtLiquidation:              collateralizationRatio,
		CurrentCollaterlisationRatio: collateralizationRatio,
		CollateralToBeAuctioned:      nil,
		LiquidationTimestamp:         time.Now(),
		SellOffHistory:               nil,
	}

	k.SetLockedVault(ctx, value)
	k.SetLockedVaultID(ctx, lockedVaultId+1)*/

	//Create a new Data Structure with the current Params
	//Set nil for all the values not available right now
	//New function will loop over locked vaults to set all values so that they can be auctioned, seperately
	//Auction will then use the selloff amount set by lockedvault function to update params .
	//Unliquidate will take place after all the events trigger.
	//
	return nil

}

func (k Keeper) CreateLockedVaultHistoy(ctx sdk.Context, lockedvault types.LockedVault) error {

	lockedVaultId := k.GetLockedVaultIDHistory(ctx)
	k.SetLockedVaultHistory(ctx, lockedvault, lockedVaultId)
	k.SetLockedVaultIDHistory(ctx, lockedVaultId+1)

	return nil

}

//for first time to update the collateralization value & sell off amount
//and if auction is complete and cr is less than 1.6
/*func (k Keeper) UpdateLockedVaults(ctx sdk.Context) error {
	lockedVaults := k.GetLockedVaults(ctx)
	if len(lockedVaults) == 0 {
		return nil
	}

	for _, lockedVault := range lockedVaults {

		pair, found := k.GetPair(ctx, lockedVault.PairId)
		if !found {
			continue
		}

		auctionParams := k.GetAuctionParams(ctx)

		unliquidatePointPercentage := pair.UnliquidationRatio

		auctionDiscount := sdk.MustNewDecFromStr(auctionParams.AuctionDiscountPercent)
		liquidationPenalty := sdk.MustNewDecFromStr(auctionParams.LiquidationPenaltyPercent)
		if (!lockedVault.IsAuctionInProgress && !lockedVault.IsAuctionComplete) || (lockedVault.IsAuctionComplete && lockedVault.CurrentCollaterlisationRatio.LTE(unliquidatePointPercentage)) {

			assetIn, found := k.GetAsset(ctx, pair.AssetIn)
			if !found {
				continue
			}
			assetOut, found := k.GetAsset(ctx, pair.AssetOut)
			if !found {
				continue
			}
			collateralizationRatio, err := k.CalculateCollaterlizationRatio(ctx, lockedVault.AmountIn, assetIn, lockedVault.AmountOut, assetOut)
			if err != nil {
				continue
			}
			//Asset Price in Dollar Terms to find how how much is to be auctioned
			assetInPrice, _ := k.GetPriceForAsset(ctx, assetIn.Id)
			assetOutPrice, _ := k.GetPriceForAsset(ctx, assetOut.Id)

			totalIn := lockedVault.AmountIn.Mul(sdk.NewIntFromUint64(assetInPrice)).ToDec()
			totalOut := lockedVault.AmountOut.Mul(sdk.NewIntFromUint64(assetOutPrice)).ToDec()
			//Selloff Collateral Calculation
			//Assuming that the collateral to be sold is 1 unit, so finding out how much is going to be deducted from the
			//collateral which will account as repaying the user's debt
			deductionPercentage, _ := sdk.NewDecFromStr("1.0")
			auctionDeduction := (deductionPercentage).Sub(auctionDiscount.Add(liquidationPenalty))
			multiplicationFactor := auctionDeduction.Mul(unliquidatePointPercentage)
			asssetOutMultiplicationFactor := totalOut.Mul(unliquidatePointPercentage)
			assetsDifference := totalIn.Sub(asssetOutMultiplicationFactor)
			//Substracting again from 1 unit to find the selloff multiplication factor
			selloffMultiplicationFactor := deductionPercentage.Sub(multiplicationFactor)
			selloffAmount := assetsDifference.Quo(selloffMultiplicationFactor)

			var collateralToBeAuctioned sdk.Dec

			//Considering a case with sudden pump & dump resulting in sudden liquidation of vaults
			//Unlocking those account without deducting any fees.

			// baseValue := sdk.ZeroInt()
			// if selloffAmount.LT(baseValue.ToDec())  {
			// 	//Unlocking User's Vault
			// 	//Very less likely scenario , but a possible edge case , nothing is impossible in Blockchain
			// 	userAddress, err := sdk.AccAddressFromBech32(lockedVault.Owner)
			// 	if err != nil {
			// 		continue
			// 	}

			// 	k.DeleteVaultForAddressByPair(ctx, userAddress, lockedVault.PairId)
			// 	k.CreteNewVault(ctx, lockedVault.PairId, lockedVault.Owner, assetIn, lockedVault.AmountIn, assetOut, lockedVault.AmountOut)
			// 	k.DeleteLockedVault(ctx, lockedVault.LockedVaultId)

			// }

			if selloffAmount.GTE(totalIn) {
				collateralToBeAuctioned = totalIn
			} else {

				collateralToBeAuctioned = selloffAmount
			}
			updatedLockedVault := lockedVault
			updatedLockedVault.CurrentCollaterlisationRatio = collateralizationRatio
			updatedLockedVault.CollateralToBeAuctioned = &collateralToBeAuctioned
			k.SetLockedVault(ctx, updatedLockedVault)

		}

	}
	return nil
}
*/
/*func (k Keeper) UnliquidateLockedVaults(ctx sdk.Context) error {
	lockedVaults := k.GetLockedVaults(ctx)
	if len(lockedVaults) == 0 {
		return nil
	}
	for _, lockedVault := range lockedVaults {

		if lockedVault.IsAuctionComplete {
			//also calculate the current collaterlization ration to ensure there is no sudden changes
			userAddress, err := sdk.AccAddressFromBech32(lockedVault.Owner)
			if err != nil {
				continue
			}

			pair, found := k.GetPair(ctx, lockedVault.PairId)
			if !found {
				continue
			}

			unliquidatePointPercentage := pair.UnliquidationRatio

			assetIn, found := k.GetAsset(ctx, pair.AssetIn)
			if !found {
				continue
			}
			assetOut, found := k.GetAsset(ctx, pair.AssetOut)
			if !found {
				continue
			}
			if lockedVault.AmountOut.IsZero() {
				k.CreateLockedVaultHistoy(ctx, lockedVault)
				k.DeleteVaultForAddressByPair(ctx, userAddress, lockedVault.PairId)
				k.DeleteLockedVault(ctx, lockedVault.LockedVaultId)
				if err := k.SendCoinFromModuleToAccount(ctx, vaulttypes.ModuleName, userAddress, sdk.NewCoin(assetIn.Denom, lockedVault.AmountIn)); err != nil {
					continue
				}
				continue
			}
			newCalculatedCollateralizationRatio, err := k.CalculateCollaterlizationRatio(ctx, lockedVault.AmountIn, assetIn, lockedVault.AmountOut, assetOut)
			if err != nil {
				continue
			}
			if newCalculatedCollateralizationRatio.LT(unliquidatePointPercentage) {
				updatedLockedVault := lockedVault
				updatedLockedVault.CurrentCollaterlisationRatio = newCalculatedCollateralizationRatio
				k.SetLockedVault(ctx, updatedLockedVault)
				continue
			}
			if newCalculatedCollateralizationRatio.GTE(unliquidatePointPercentage) {
				k.CreateLockedVaultHistoy(ctx, lockedVault)
				k.DeleteVaultForAddressByPair(ctx, userAddress, lockedVault.PairId)
				k.CreteNewVault(ctx, lockedVault.PairId, lockedVault.Owner, assetIn, lockedVault.AmountIn, assetOut, lockedVault.AmountOut)
				k.DeleteLockedVault(ctx, lockedVault.LockedVaultId)

				//======================================NOTE TO BE CHANGED================================================
				//One important thing that we missed is that we need to pop and append the current vault as per the user -> This has bee handled -Vishnu
				//IF all the borrowed amount is repayed , then we need to ensure the unliquidate vault is not called for that particular lockedvault- his vault is automatically closed.
			}
		}
	}

	return nil
}
*/
func (k Keeper) GetModAccountBalances(ctx sdk.Context, accountName string, denom string) sdk.Int {
	macc := k.GetModuleAccount(ctx, accountName)
	return k.GetBalance(ctx, macc.GetAddress(), denom).Amount
}

func (k *Keeper) GetLockedVaultIDbyApp(ctx sdk.Context, appId uint64) uint64 {
	var (
		store = k.Store(ctx)
		key   = types.AppLockedVaultMappingKey(appId)
		value = store.Get(key)
	)

	if value == nil {
		return 0
	}

	var id protobuftypes.UInt64Value
	k.cdc.MustUnmarshal(value, &id)

	return id.GetValue()
}

func (k *Keeper) GetLockedVaultIDHistory(ctx sdk.Context) uint64 {
	var (
		store = k.Store(ctx)
		key   = types.LockedVaultKeyHistory
		value = store.Get(key)
	)

	if value == nil {
		return 0
	}

	var id protobuftypes.UInt64Value
	k.cdc.MustUnmarshal(value, &id)

	return id.GetValue()
}

func (k *Keeper) SetLockedVaultIDbyApp(ctx sdk.Context, id uint64) {
	var (
		store = k.Store(ctx)
		key   = types.AppLockedVaultMappingKey(id)
		value = k.cdc.MustMarshal(
			&protobuftypes.UInt64Value{
				Value: id,
			},
		)
	)
	store.Set(key, value)
}

func (k *Keeper) SetLockedVaultIDHistory(ctx sdk.Context, id uint64) {
	var (
		store = k.Store(ctx)
		key   = types.LockedVaultKeyHistory
		value = k.cdc.MustMarshal(
			&protobuftypes.UInt64Value{
				Value: id,
			},
		)
	)
	store.Set(key, value)
}

func (k *Keeper) SetLockedVault(ctx sdk.Context, locked_vault types.LockedVault) {
	var (
		store = k.Store(ctx)
		key   = types.LockedVaultKey(locked_vault.LockedVaultId)
		value = k.cdc.MustMarshal(&locked_vault)
	)
	store.Set(key, value)
}

func (k *Keeper) SetLockedVaultHistory(ctx sdk.Context, locked_vault types.LockedVault, id uint64) {
	var (
		store = k.Store(ctx)
		key   = types.LockedVaultHistoryKey(id)
		value = k.cdc.MustMarshal(&locked_vault)
	)
	store.Set(key, value)
}

func (k *Keeper) DeleteLockedVault(ctx sdk.Context, id uint64) {
	var (
		store = k.Store(ctx)
		key   = types.LockedVaultKey(id)
	)
	store.Delete(key)
}

func (k *Keeper) GetLockedVault(ctx sdk.Context, id uint64) (locked_vault types.LockedVault, found bool) {
	var (
		store = k.Store(ctx)
		key   = types.LockedVaultKey(id)
		value = store.Get(key)
	)

	if value == nil {
		return locked_vault, false
	}

	k.cdc.MustUnmarshal(value, &locked_vault)
	return locked_vault, true
}

func (k *Keeper) GetLockedVaults(ctx sdk.Context) (locked_vaults []types.LockedVault) {
	var (
		store = k.Store(ctx)
		iter  = sdk.KVStorePrefixIterator(store, types.LockedVaultKeyPrefix)
	)

	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var locked_vault types.LockedVault
		k.cdc.MustUnmarshal(iter.Value(), &locked_vault)
		locked_vaults = append(locked_vaults, locked_vault)
	}

	return locked_vaults
}

func (k *Keeper) SetFlagIsAuctionInProgress(ctx sdk.Context, id uint64, flag bool) error {

	locked_vault, found := k.GetLockedVault(ctx, id)
	if !found {
		return types.LockedVaultDoesNotExist
	}
	locked_vault.IsAuctionInProgress = flag
	k.SetLockedVault(ctx, locked_vault)
	return nil
}

func (k *Keeper) SetFlagIsAuctionComplete(ctx sdk.Context, id uint64, flag bool) error {

	locked_vault, found := k.GetLockedVault(ctx, id)
	if !found {
		return types.LockedVaultDoesNotExist
	}
	locked_vault.IsAuctionComplete = flag
	k.SetLockedVault(ctx, locked_vault)
	return nil
}

/*func (k *Keeper) UpdateAssetQuantitiesInLockedVault(
	ctx sdk.Context,
	collateral_auction auctiontypes.CollateralAuction,
	amountIn sdk.Int,
	assetIn assettypes.Asset,
	amountOut sdk.Int,
	assetOut assettypes.Asset,
) error {

	locked_vault, found := k.GetLockedVault(ctx, collateral_auction.LockedVaultId)
	if !found {
		return types.LockedVaultDoesNotExist
	}
	updatedAmountIn := locked_vault.AmountIn.Sub(amountIn)
	updatedAmountOut := locked_vault.AmountOut.Sub(amountOut)
	updatedCollateralizationRatio, _ := k.CalculateCollaterlizationRatio(ctx, updatedAmountIn, assetIn, updatedAmountOut, assetOut)

	locked_vault.AmountIn = updatedAmountIn
	locked_vault.AmountOut = updatedAmountOut
	locked_vault.CurrentCollaterlisationRatio = updatedCollateralizationRatio
	locked_vault.SellOffHistory = append(locked_vault.SellOffHistory, collateral_auction.String())
	k.SetLockedVault(ctx, locked_vault)
	return nil
}*/

func (k *Keeper) SetAppId(ctx sdk.Context, AppIds types.WhitelistedAppIds) {
	var (
		store = k.Store(ctx)
		key   = types.AppIdsKeyPrefix
		value = k.cdc.MustMarshal(&AppIds)
	)

	store.Set(key, value)
}

func (k *Keeper) GetAppIds(ctx sdk.Context) (appIds types.WhitelistedAppIds) {
	var (
		store = k.Store(ctx)
		key   = types.AppIdsKeyPrefix
		value = store.Get(key)
	)

	if value == nil {
		return appIds
	}

	k.cdc.MustUnmarshal(value, &appIds)
	return appIds
}
