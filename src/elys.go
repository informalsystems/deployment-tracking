package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
)

const (
	UsdcRewardDenomForQuery = "ibc%2FF082B65C88E4B6D5EF1DB243CDA1D331D002759E938A0F5CD3FFDC5D53B3E349"
	UedenRewardDenom        = "ueden"
	UsdcPoolId              = math.MaxInt16
)

type PoolType string

const (
	Stablestake PoolType = "stablestake"
	AMM         PoolType = "amm"
)

type ElysVenuePositionConfig struct {
	PoolId       string
	Address      string
	ActiveShares float64  // lp token amount, this is a way to track the funds deployed per bid
	PoolType     PoolType // Enum to specify the pool type
}

func (venueConfig ElysVenuePositionConfig) GetProtocol() Protocol {
	return Elys
}

func (venueConfig ElysVenuePositionConfig) GetPoolID() string {
	return venueConfig.PoolId
}

func (venueConfig ElysVenuePositionConfig) GetAddress() string {
	return venueConfig.Address
}

type ElysPosition struct {
	protocolConfig      ProtocolConfig
	venuePositionConfig ElysVenuePositionConfig
}

func NewElysPosition(config ProtocolConfig, venuePositionConfig VenuePositionConfig) (*ElysPosition, error) {
	ElysVenuePositionConfig, ok := venuePositionConfig.(ElysVenuePositionConfig)
	if !ok {
		return nil, fmt.Errorf("venuePositionConfig must be of ElysVenuePositionConfig type")
	}

	return &ElysPosition{
		protocolConfig:      config,
		venuePositionConfig: ElysVenuePositionConfig,
	}, nil
}

func (p ElysPosition) ComputeTVL(assetData *ChainInfo) (*Holdings, error) {
	switch p.venuePositionConfig.PoolType {
	case Stablestake:
		return p.computeStablestakeTVL(assetData)
	case AMM:
		return p.computeAMMTVL(assetData)
	default:
		return nil, fmt.Errorf("unsupported pool type: %s", p.venuePositionConfig.PoolType)
	}
}

func (p ElysPosition) computeStablestakeTVL(assetData *ChainInfo) (*Holdings, error) {
	poolData, err := p.fetchStablestakePoolData()
	if err != nil {
		return nil, err
	}

	pool, ok := poolData["pool"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid pool data")
	}

	depositDenom, ok := pool["deposit_denom"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid deposit_denom in pool data")
	}

	netAmountStr, ok := pool["net_amount"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid net_amount in pool data")
	}

	amount, err := strconv.ParseInt(netAmountStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing net_amount: %v", err)
	}

	tokenInfo, err := assetData.GetTokenInfo(depositDenom)
	if err != nil {
		return nil, fmt.Errorf("getting token info: %v", err)
	}

	adjustedAmount := float64(amount) / math.Pow(10, float64(tokenInfo.Decimals))

	usdValue, atomValue, err := getTokenValues(adjustedAmount, *tokenInfo)
	if err != nil {
		return nil, fmt.Errorf("calculating token values: %v", err)
	}

	poolAssets := []Asset{
		{
			Denom:       depositDenom,
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

func (p ElysPosition) computeAMMTVL(assetData *ChainInfo) (*Holdings, error) {
	// Fetch AMM pool data
	poolData, err := p.fetchAMMPoolData()
	if err != nil {
		return nil, fmt.Errorf("fetching AMM pool data: %v", err)
	}

	// Extract pool assets
	pool, ok := poolData["pool"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid pool data")
	}

	poolAssetsData, ok := pool["pool_assets"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid pool_assets in pool data")
	}

	var poolAssets []Asset
	var usdTotal, atomTotal float64

	// Iterate through each token in the pool
	for _, asset := range poolAssetsData {
		poolAsset, ok := asset.(map[string]interface{}) // Renamed to poolAsset
		if !ok {
			return nil, fmt.Errorf("invalid asset data format")
		}

		token, ok := poolAsset["token"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("missing or invalid token data in asset")
		}

		amountStr, ok := token["amount"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid amount in token data")
		}

		denom, ok := token["denom"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid denom in token data")
		}

		// Parse the amount
		amount, err := strconv.ParseInt(amountStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing token amount: %v", err)
		}

		// Get token info
		tokenInfo, err := assetData.GetTokenInfo(denom)
		if err != nil {
			return nil, fmt.Errorf("getting token info for denom %s: %v", denom, err)
		}

		// Calculate adjusted amount
		adjustedAmount := float64(amount) / math.Pow(10, float64(tokenInfo.Decimals))

		// Calculate USD and ATOM values
		usdValue, atomValue, err := getTokenValues(adjustedAmount, *tokenInfo)
		if err != nil {
			return nil, fmt.Errorf("calculating token values for denom %s: %v", denom, err)
		}

		// Add to total values
		usdTotal += usdValue
		atomTotal += atomValue

		// Add to pool assets
		poolAssets = append(poolAssets, Asset{
			Denom:       denom,
			Amount:      adjustedAmount,
			USDValue:    usdValue,
			DisplayName: tokenInfo.Display,
		})
	}

	// Return holdings
	return &Holdings{
		Balances:  poolAssets,
		TotalUSDC: usdTotal,
		TotalAtom: atomTotal,
	}, nil
}

func (p ElysPosition) ComputeAddressPrincipalHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	if p.venuePositionConfig.ActiveShares == 0 {
		return &Holdings{
			Balances:  []Asset{},
			TotalUSDC: 0,
			TotalAtom: 0,
		}, nil
	}

	switch p.venuePositionConfig.PoolType {
	case Stablestake:
		return p.computeStablestakePrincipalHoldings(assetData, address)
	case AMM:
		return p.computeAMMPrincipalHoldings(assetData, address)
	default:
		return nil, fmt.Errorf("unsupported pool type: %s", p.venuePositionConfig.PoolType)
	}
}

func (p ElysPosition) computeStablestakePrincipalHoldings(assetData *ChainInfo, _ string) (*Holdings, error) {
	amount := p.venuePositionConfig.ActiveShares

	poolData, err := p.fetchStablestakePoolData()
	if err != nil {
		return nil, err
	}

	pool, ok := poolData["pool"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid pool data")
	}

	redemptionRateStr, ok := pool["redemption_rate"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid redemption_rate in pool data")
	}

	redemptionRate, err := strconv.ParseFloat(redemptionRateStr, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing redemption_rate: %v", err)
	}

	depositDenom, ok := pool["deposit_denom"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid deposit_denom in pool data")
	}

	tokenInfo, err := assetData.GetTokenInfo(depositDenom)
	if err != nil {
		return nil, fmt.Errorf("getting token info: %v", err)
	}

	adjustedAmount := float64(amount) / math.Pow(10, float64(tokenInfo.Decimals))
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

func (p ElysPosition) computeAMMPrincipalHoldings(assetData *ChainInfo, _ string) (*Holdings, error) {
	// Use LPAmount from the venue position config
	amount := p.venuePositionConfig.ActiveShares
	if amount == 0 {
		return nil, fmt.Errorf("LPAmount is zero, no holdings to compute")
	}

	// Fetch AMM pool data
	poolData, err := p.fetchAMMPoolData()
	if err != nil {
		return nil, fmt.Errorf("fetching AMM pool data: %v", err)
	}

	// Extract lp_token_price from the pool data
	extraInfo, ok := poolData["extra_info"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid extra_info in AMM pool data")
	}

	lpTokenPriceStr, ok := extraInfo["lp_token_price"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid lp_token_price in AMM pool data")
	}

	lpTokenPrice, err := strconv.ParseFloat(lpTokenPriceStr, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing lp_token_price: %v", err)
	}

	// the total price of the LP tokens in USD (we assume for all pools is expressed in USDC?)
	usdcDenom := "ibc/F082B65C88E4B6D5EF1DB243CDA1D331D002759E938A0F5CD3FFDC5D53B3E349"

	// Get token info for the deposited denom
	tokenInfo, err := assetData.GetTokenInfo(usdcDenom)
	if err != nil {
		return nil, fmt.Errorf("getting token info: %v", err)
	}

	// Calculate holdings
	// This share is expressed in 10**18 units and it's the share of the pool.
	// It can be multiplied by the LP token price to understand the USD position value.
	adjustedAmount := float64(amount) / math.Pow(10, 18)
	holdings := adjustedAmount * lpTokenPrice

	// Calculate USD and ATOM values
	usdValue, atomValue, err := getTokenValues(holdings, *tokenInfo)
	if err != nil {
		return nil, fmt.Errorf("calculating token values: %v", err)
	}

	// Create holding assets
	holdingAssets := []Asset{
		{
			Denom:       usdcDenom,
			Amount:      holdings,
			USDValue:    usdValue,
			DisplayName: tokenInfo.Display,
		},
	}

	// Return holdings
	return &Holdings{
		Balances:  holdingAssets,
		TotalUSDC: usdValue,
		TotalAtom: atomValue,
	}, nil
}

// We can only calculate rewards per address, not per bid.
func (p ElysPosition) ComputeAddressRewardHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	if p.venuePositionConfig.ActiveShares == 0 {
		return &Holdings{
			Balances:  []Asset{},
			TotalUSDC: 0,
			TotalAtom: 0,
		}, nil
	}

	rewardDenoms := []string{UsdcRewardDenomForQuery, UedenRewardDenom}

	var rewardAssets []Asset
	totalValueUSD := 0.0
	totalValueATOM := 0.0

	for _, queryDenom := range rewardDenoms {
		rewardURL := fmt.Sprintf("%s/masterchef/user_reward_info?user=%s&pool_id=%s&reward_denom=%s",
			p.protocolConfig.PoolInfoUrl, address, p.venuePositionConfig.PoolId, queryDenom)

		resp, err := http.Get(rewardURL)
		if err != nil {
			debugLog("Error fetching reward data", map[string]string{"denom": queryDenom, "error": err.Error()})
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			debugLog("Error fetching reward data: invalid status code", map[string]string{
				"denom":  queryDenom,
				"status": strconv.Itoa(resp.StatusCode),
			})
			continue
		}

		var rewardData map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&rewardData); err != nil {
			debugLog("Error decoding reward response", map[string]string{"denom": queryDenom, "error": err.Error()})
			continue
		}

		userRewardInfo, ok := rewardData["user_reward_info"].(map[string]interface{})
		if !ok {
			debugLog("Missing or invalid user_reward_info in reward data", map[string]string{"denom": queryDenom})
			continue
		}

		rewardDenom, ok := userRewardInfo["reward_denom"].(string)
		if !ok {
			debugLog("Missing or invalid reward_denom in reward data", map[string]string{"denom": queryDenom})
			continue
		}

		rewardPendingStr, ok := userRewardInfo["reward_pending"].(string)
		if !ok {
			debugLog("Missing or invalid reward_pending in reward data", map[string]string{"denom": rewardDenom})
			continue
		}

		rewardPending, err := strconv.ParseFloat(rewardPendingStr, 64)
		if err != nil {
			debugLog("Error parsing reward_pending amount", map[string]string{"denom": rewardDenom, "error": err.Error()})
			continue
		}

		tokenInfo, err := assetData.GetTokenInfo(rewardDenom)
		if err != nil {
			debugLog("Token info not found", map[string]string{"denom": rewardDenom})
			continue
		}

		adjustedAmount := rewardPending / math.Pow(10, float64(tokenInfo.Decimals))
		usdValue, atomValue, err := getTokenValues(adjustedAmount, *tokenInfo)
		if err != nil {
			debugLog("Error getting token values", map[string]string{"denom": rewardDenom})
			continue
		}

		totalValueUSD += usdValue
		totalValueATOM += atomValue

		rewardAssets = append(rewardAssets, Asset{
			Denom:       rewardDenom,
			Amount:      adjustedAmount,
			USDValue:    usdValue,
			DisplayName: tokenInfo.Display,
		})
	}

	return &Holdings{
		Balances:  rewardAssets,
		TotalUSDC: totalValueUSD,
		TotalAtom: totalValueATOM,
	}, nil
}

func (p ElysPosition) fetchStablestakePoolData() (map[string]interface{}, error) {
	poolURL := fmt.Sprintf("%s/stablestake/pool/%s", p.protocolConfig.PoolInfoUrl, p.venuePositionConfig.PoolId)

	resp, err := http.Get(poolURL)
	if err != nil {
		return nil, fmt.Errorf("fetching stablestake pool info: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching stablestake pool info: status %d", resp.StatusCode)
	}

	var poolData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&poolData); err != nil {
		return nil, fmt.Errorf("decoding stablestake pool response: %v", err)
	}

	return poolData, nil
}

func (p ElysPosition) fetchAMMPoolData() (map[string]interface{}, error) {
	// Construct the URL for querying the AMM pool
	poolURL := fmt.Sprintf("%s/amm/pool/%s/%s", p.protocolConfig.PoolInfoUrl, p.venuePositionConfig.PoolId, "1")

	// Make the HTTP GET request
	resp, err := http.Get(poolURL)
	if err != nil {
		return nil, fmt.Errorf("fetching AMM pool info: %v", err)
	}
	defer resp.Body.Close()

	// Check for a valid HTTP status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching AMM pool info: status %d", resp.StatusCode)
	}

	// Decode the JSON response into a map
	var poolData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&poolData); err != nil {
		return nil, fmt.Errorf("decoding AMM pool response: %v", err)
	}

	return poolData, nil
}
