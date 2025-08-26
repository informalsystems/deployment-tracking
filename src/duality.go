package main

import (
	"fmt"
	"math"
	"strconv"
)

type DualityVenuePositionConfig struct {
	PoolAddress  string // Contract address of the pool
	Address      string
	ActiveShares int64 // LP token amount, this is a way to track the funds deployed per bid
}

func (venueConfig DualityVenuePositionConfig) GetProtocol() Protocol {
	return Duality
}

func (venueConfig DualityVenuePositionConfig) GetPoolID() string {
	return venueConfig.PoolAddress
}

func (venueConfig DualityVenuePositionConfig) GetAddress() string {
	return venueConfig.Address
}

type DualityPosition struct {
	protocolConfig      ProtocolConfig
	venuePositionConfig DualityVenuePositionConfig
}

func NewDualityPosition(config ProtocolConfig, venuePositionConfig VenuePositionConfig) (*DualityPosition, error) {
	dualityVenuePositionConfig, ok := venuePositionConfig.(DualityVenuePositionConfig)
	if !ok {
		return nil, fmt.Errorf("venuePositionConfig must be of DualityVenuePositionConfig type")
	}

	return &DualityPosition{
		protocolConfig:      config,
		venuePositionConfig: dualityVenuePositionConfig,
	}, nil
}

func (p DualityPosition) ComputeTVL(assetData *ChainInfo) (*Holdings, error) {
	// Query pool info
	queryMsg := map[string]interface{}{
		"get_balance": map[string]interface{}{},
	}

	poolData, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl,
		p.venuePositionConfig.PoolAddress, queryMsg)
	if err != nil {
		return nil, fmt.Errorf("querying pool data: %w", err)
	}

	// Handle case where poolData is a list of tokens
	pairDataList, ok := poolData.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected poolData to be a list")
	}

	var poolAssets []Asset
	totalValueUSD := 0.0
	totalValueATOM := 0.0

	for _, entry := range pairDataList {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			debugLog("Invalid entry in pool data", nil)
			continue
		}

		// Extract denom
		denom, ok := entryMap["denom"].(string)
		if !ok {
			debugLog("Missing denom in pool entry", nil)
			continue
		}

		// Extract amount
		amountStr, ok := entryMap["amount"].(string)
		if !ok {
			debugLog("Missing amount in pool entry", map[string]string{"denom": denom})
			continue
		}

		amount, err := strconv.ParseInt(amountStr, 10, 64)
		if err != nil {
			debugLog("Error parsing amount", map[string]string{"denom": denom})
			continue
		}

		// Get token info
		tokenInfo, err := assetData.GetTokenInfo(denom)
		if err != nil {
			debugLog("Token info not found", map[string]string{"denom": denom})
			continue
		}

		// Adjust amount by decimals
		adjustedAmount := float64(amount) / math.Pow(10, float64(tokenInfo.Decimals))

		// Get USD and ATOM value
		usdValue, atomValue, err := getTokenValues(adjustedAmount, *tokenInfo)
		if err != nil {
			debugLog("Error getting token values", map[string]string{"denom": denom})
			continue
		}

		totalValueUSD += usdValue
		totalValueATOM += atomValue

		// Append to results
		poolAssets = append(poolAssets, Asset{
			Denom:       denom,
			Amount:      adjustedAmount,
			USDValue:    usdValue,
			DisplayName: tokenInfo.Display,
		})
	}

	return &Holdings{
		Balances:  poolAssets,
		TotalUSDC: totalValueUSD,
		TotalAtom: totalValueATOM,
	}, nil
}

func (p DualityPosition) ComputeAddressPrincipalHoldings(assetData *ChainInfo, _ string) (*Holdings, error) {
	// Use LPAmount from the venue position config
	totalLPAmount := p.venuePositionConfig.ActiveShares

	// Check what share of the pool the LP amounts correspond to
	withdrawQuery := map[string]interface{}{
		"simulate_withdraw_liquidity": map[string]interface{}{
			"amount": strconv.FormatInt(totalLPAmount, 10),
		},
	}

	withdrawData, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl,
		p.venuePositionConfig.PoolAddress, withdrawQuery)
	if err != nil {
		return nil, fmt.Errorf("simulating withdrawal: %s", err)
	}

	// Directly cast the response to []interface{}
	amounts, ok := withdrawData.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected data format: expected an array of token amounts")
	}

	if len(amounts) != 2 {
		return nil, fmt.Errorf("unexpected data format: expected 2 token amounts")
	}

	// Get pool assets for token denominations
	poolAssets, err := getPoolAssets(p)
	if err != nil {
		return nil, fmt.Errorf("getting pool assets: %s", err)
	}

	var holdingAssets []Asset
	totalValueUSD := 0.0
	totalValueATOM := 0.0

	for i, amountStr := range amounts {
		amount, err := strconv.ParseInt(amountStr.(string), 10, 64)
		if err != nil {
			debugLog("Error parsing token amount", map[string]string{"index": strconv.Itoa(i)})
			continue
		}

		denom := poolAssets[i].Denom
		tokenInfo, err := assetData.GetTokenInfo(denom)
		if err != nil {
			debugLog("Token info not found", map[string]string{"denom": denom})
			continue
		}

		adjustedAmount := float64(amount) / math.Pow(10, float64(tokenInfo.Decimals))
		usdValue, atomValue, err := getTokenValues(adjustedAmount, *tokenInfo)
		if err != nil {
			debugLog("Error getting token values", map[string]string{"denom": denom})
			continue
		}

		totalValueUSD += usdValue
		totalValueATOM += atomValue

		holdingAssets = append(holdingAssets, Asset{
			Denom:       denom,
			Amount:      adjustedAmount,
			USDValue:    usdValue,
			DisplayName: tokenInfo.Display,
		})
	}

	return &Holdings{
		Balances:  holdingAssets,
		TotalUSDC: totalValueUSD,
		TotalAtom: totalValueATOM,
	}, nil
}

func (p DualityPosition) ComputeAddressRewardHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	// Duality protocol doesn't keep track of the initial holdings and yield separately
	return &Holdings{}, nil
}

func getPoolAssets(p DualityPosition) ([]Asset, error) {
	configQuery := map[string]interface{}{
		"get_config": map[string]interface{}{},
	}

	configData, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl,
		p.venuePositionConfig.PoolAddress, configQuery)
	if err != nil {
		return nil, fmt.Errorf("querying pool config: %s", err)
	}

	// Validate configData structure
	configMap, ok := configData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid configData format: expected map[string]interface{}")
	}

	// Validate pair_data presence and type
	pairData, ok := configMap["pair_data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid pair_data in configData")
	}

	// Validate token_0 and token_1 denominations
	token0, ok := pairData["token_0"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid token_0 in pair_data")
	}
	token1, ok := pairData["token_1"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid token_1 in pair_data")
	}

	token0Denom, ok := token0["denom"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid denom in token_0")
	}
	token1Denom, ok := token1["denom"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid denom in token_1")
	}

	// Create Asset objects for token_0 and token_1
	token0Asset := Asset{
		Denom: token0Denom,
	}
	token1Asset := Asset{
		Denom: token1Denom,
	}

	return []Asset{token0Asset, token1Asset}, nil
}
