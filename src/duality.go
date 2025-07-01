package main

import (
	"fmt"
	"math"
	"strconv"
)

type DualityVenuePositionConfig struct {
	PoolAddress string // Contract address of the pool
	Address     string
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
		return nil, fmt.Errorf("querying pool data: %s", err)
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

func (p DualityPosition) ComputeAddressPrincipalHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	// First get LP token info
	lpToken, err := getLPToken(p)
	if err != nil {
		return nil, err
	}

	// Get user's LP balance
	totalLPAmount, err := p.getLPAmount(address, lpToken)
	if err != nil {
		return nil, fmt.Errorf("getting total LP amount: %s", err)
	}

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

	// Directly cast the response to []interface{}s
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

func getLPToken(p DualityPosition) (string, error) {
	configQuery := map[string]interface{}{
		"get_config": map[string]interface{}{},
	}

	configData, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl,
		p.venuePositionConfig.PoolAddress, configQuery)
	if err != nil {
		return "", fmt.Errorf("querying pool config: %s", err)
	}

	lpToken := (configData.(map[string]interface{}))["lp_denom"].(string)
	return lpToken, nil
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

	// Extract token_0 and token_1 denominations
	pairData := configData.(map[string]interface{})["pair_data"].(map[string]interface{})
	token0Denom := pairData["token_0"].(map[string]interface{})["denom"].(string)
	token1Denom := pairData["token_1"].(map[string]interface{})["denom"].(string)

	// Create Asset objects for token_0 and token_1
	token0 := Asset{
		Denom: token0Denom,
	}
	token1 := Asset{
		Denom: token1Denom,
	}

	return []Asset{token0, token1}, nil
}

func (p DualityPosition) getLPAmount(address string, lpToken string) (int64, error) {
	walletBalance := int64(0)

	balanceURL := fmt.Sprintf("%s/%s", p.protocolConfig.AddressBalanceUrl, address)
	var balanceResponse struct {
		Balances []struct {
			Denom  string `json:"denom"`
			Amount string `json:"amount"`
		} `json:"balances"`
	}

	if err := getJSON(balanceURL, &balanceResponse); err != nil {
		return 0, fmt.Errorf("querying balance: %s", err)
	}

	// Try to find as native token
	for _, bal := range balanceResponse.Balances {
		if bal.Denom == lpToken {
			walletBalance, _ = strconv.ParseInt(bal.Amount, 10, 64)
			break
		}
	}

	return walletBalance, nil
}
