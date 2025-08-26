package main

import (
	"fmt"
	"math"
	"strconv"
)

const (
	INJECTIVE_ATOM     = "ibc/C4CFF46FD6DE35CA4CF4CE031E643C8FDC9BA4B99AE598E9B0ED98FE3A2319F9"
	MarketMakerAddress = "inj1nc7gjkf2mhp34a6gquhurg8qahnw5kxs5u3s4u"
)

type NeptuneVenuePositionConfig struct {
	Denom        string
	Address      string
	ActiveShares int64 // LP token amount
}

func (venueConfig NeptuneVenuePositionConfig) GetProtocol() Protocol {
	return Neptune
}

func (venueConfig NeptuneVenuePositionConfig) GetPoolID() string {
	return MarketMakerAddress
}

func (venueConfig NeptuneVenuePositionConfig) GetAddress() string {
	return venueConfig.Address
}

type NeptunePosition struct {
	protocolConfig      ProtocolConfig
	venuePositionConfig NeptuneVenuePositionConfig
}

func NewNeptunePosition(config ProtocolConfig, venuePositionConfig VenuePositionConfig) (*NeptunePosition, error) {
	NeptuneVenuePositionConfig, ok := venuePositionConfig.(NeptuneVenuePositionConfig)
	if !ok {
		return nil, fmt.Errorf("venuePositionConfig must be of NeptuneVenuePositionConfig type")
	}

	return &NeptunePosition{
		protocolConfig:      config,
		venuePositionConfig: NeptuneVenuePositionConfig,
	}, nil
}

func (p NeptunePosition) ComputeTVL(assetData *ChainInfo) (*Holdings, error) {
	amount, err := p.getPoolLentAmount()
	if err != nil {
		return nil, fmt.Errorf("error getting pool lent amount: %v", err)
	}

	denom := p.venuePositionConfig.Denom

	tokenInfo, err := assetData.GetTokenInfo(denom)
	if err != nil {
		debugLog("Token info not found", map[string]string{"denom": denom})
		return nil, fmt.Errorf("error getting token info for denom: %s", denom)
	}

	adjustedAmount := float64(amount) / math.Pow(10, float64(tokenInfo.Decimals))

	usdValue, atomValue, err := getTokenValues(adjustedAmount, *tokenInfo)
	if err != nil {
		debugLog("Error getting token values", map[string]string{"denom": denom})
		return nil, fmt.Errorf("error calculating token values for denom: %s", denom)
	}

	poolAssets := []Asset{
		{
			Denom:       denom,
			Amount:      adjustedAmount,
			USDValue:    usdValue,
			DisplayName: tokenInfo.Display,
		},
	}

	return &Holdings{
		Balances:  poolAssets,
		TotalUSDC: usdValue,
		TotalAtom: atomValue,
	}, nil
}

func (p NeptunePosition) ComputeAddressPrincipalHoldings(assetData *ChainInfo, _ string) (*Holdings, error) {
	receiptAddr, err := p.getPoolReceiptToken()
	if err != nil {
		return nil, fmt.Errorf("error getting pool receipt token: %v", err)
	}

	redemptionRate, err := p.calculateRedemptionRate(receiptAddr)
	if err != nil {
		return nil, fmt.Errorf("error calculating redemption rate: %v", err)
	}

	depositDenom := p.venuePositionConfig.Denom
	tokenInfo, err := assetData.GetTokenInfo(depositDenom)
	if err != nil {
		return nil, fmt.Errorf("getting token info: %v", err)
	}

	adjustedAmount := float64(p.venuePositionConfig.ActiveShares) / math.Pow(10, float64(tokenInfo.Decimals))
	holdings := adjustedAmount * redemptionRate

	usdValue, atomValue, err := getTokenValues(holdings, *tokenInfo)
	if err != nil {
		return nil, fmt.Errorf("calculating token values: %v", err)
	}

	holdingAssets := []Asset{
		{
			Denom:       depositDenom,
			Amount:      holdings,
			USDValue:    usdValue,
			DisplayName: tokenInfo.Display,
		},
	}

	return &Holdings{
		Balances:  holdingAssets,
		TotalUSDC: usdValue,
		TotalAtom: atomValue,
	}, nil
}

func (p NeptunePosition) ComputeAddressRewardHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	// Neptune protocol doesn't keep track of the initial holdings and yield separately
	return &Holdings{}, nil
}

func (p NeptunePosition) getPoolLentAmount() (float64, error) {
	queryJson := map[string]interface{}{
		"get_all_markets": map[string]interface{}{},
	}

	data, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl, MarketMakerAddress, queryJson)
	if err != nil {
		return 0, fmt.Errorf("querying smart contract data: %v", err)
	}

	markets, ok := data.([]interface{})
	if !ok {
		return 0, fmt.Errorf("unexpected response format: expected an array")
	}

	for _, market := range markets {
		marketArray, ok := market.([]interface{})
		if !ok || len(marketArray) != 2 {
			continue
		}

		nativeToken, ok := marketArray[0].(map[string]interface{})["native_token"].(map[string]interface{})
		if !ok {
			continue
		}

		denom, ok := nativeToken["denom"].(string)
		if !ok || denom != p.venuePositionConfig.Denom {
			continue
		}

		lendingPrincipalStr, ok := marketArray[1].(map[string]interface{})["lending_principal"].(string)
		if !ok {
			return 0, fmt.Errorf("missing or invalid lending_principal in market data")
		}

		lendingPrincipal, err := strconv.ParseFloat(lendingPrincipalStr, 64)
		if err != nil {
			return 0, fmt.Errorf("parsing lending_principal: %v", err)
		}

		return lendingPrincipal, nil
	}

	return 0, fmt.Errorf("no matching pool found for denom: %s", p.venuePositionConfig.Denom)
}

func (p NeptunePosition) getPoolReceiptToken() (string, error) {
	queryJson := map[string]interface{}{
		"get_all_markets": map[string]interface{}{},
	}

	data, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl, MarketMakerAddress, queryJson)
	if err != nil {
		return "", fmt.Errorf("querying smart contract data: %v", err)
	}

	markets, ok := data.([]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected response format: expected an array")
	}

	for _, market := range markets {
		marketArray, ok := market.([]interface{})
		if !ok || len(marketArray) != 2 {
			continue
		}

		nativeToken, ok := marketArray[0].(map[string]interface{})["native_token"].(map[string]interface{})
		if !ok {
			continue
		}

		denom, ok := nativeToken["denom"].(string)
		if !ok || denom != p.venuePositionConfig.Denom {
			continue
		}

		marketAssetDetails, ok := marketArray[1].(map[string]interface{})["market_asset_details"].(map[string]interface{})
		if !ok {
			continue
		}

		receiptAddr, ok := marketAssetDetails["receipt_addr"].(string)
		if !ok {
			return "", fmt.Errorf("missing or invalid receipt_addr in market_asset_details")
		}

		return receiptAddr, nil
	}

	return "", fmt.Errorf("no matching pool found for denom: %s", p.venuePositionConfig.Denom)
}

func (p NeptunePosition) calculateRedemptionRate(receiptAddr string) (float64, error) {
	queryJson := map[string]interface{}{
		"token_info": map[string]interface{}{},
	}

	data, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl, receiptAddr, queryJson)
	if err != nil {
		return 0, fmt.Errorf("querying receipt token info: %v", err)
	}

	totalSupplyStr, ok := data.(map[string]interface{})["total_supply"].(string)
	if !ok {
		return 0, fmt.Errorf("missing or invalid total_supply in receipt token info")
	}

	totalSupply, err := strconv.ParseFloat(totalSupplyStr, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing total_supply: %v", err)
	}

	lendingPrincipal, err := p.getPoolLentAmount()
	if err != nil {
		return 0, fmt.Errorf("error getting pool lent amount: %v", err)
	}

	redemptionRate := lendingPrincipal / totalSupply
	return redemptionRate, nil
}
