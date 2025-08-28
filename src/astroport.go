package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type AstroportVenuePositionConfig struct {
	PoolAddress      string // Contract address of the pool
	Address          string
	IncentiveAddress string
	Protocol         Protocol
	ActiveShares     int64 // LP token amount, this is a way to track the funds deployed per bid
}

func (venueConfig AstroportVenuePositionConfig) GetProtocol() Protocol {
	return venueConfig.Protocol
}

func (venueConfig AstroportVenuePositionConfig) GetPoolID() string {
	return venueConfig.PoolAddress
}

func (venueConfig AstroportVenuePositionConfig) GetAddress() string {
	return venueConfig.Address
}

type AstroportPosition struct {
	protocolConfig      ProtocolConfig
	venuePositionConfig AstroportVenuePositionConfig
}

func NewAstroportPosition(config ProtocolConfig, venuePositionConfig VenuePositionConfig) (*AstroportPosition, error) {
	astroportVenuePositionConfig, ok := venuePositionConfig.(AstroportVenuePositionConfig)
	if !ok {
		return nil, fmt.Errorf("venuePositionConfig must be of AstroportVenuePositionConfig type")
	}

	return &AstroportPosition{
		protocolConfig:      config,
		venuePositionConfig: astroportVenuePositionConfig,
	}, nil
}

func (p AstroportPosition) ComputeTVL(assetData *ChainInfo) (*Holdings, error) {
	// Query pool info
	queryMsg := map[string]interface{}{
		"pool": map[string]interface{}{},
	}

	data, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl,
		p.venuePositionConfig.PoolAddress, queryMsg)
	if err != nil {
		return nil, fmt.Errorf("querying pool data: %s", err)
	}

	poolData := data.(map[string]interface{})

	assets := poolData["assets"].([]interface{})
	var poolAssets []Asset
	totalValueUSD := 0.0
	totalValueATOM := 0.0

	for _, asset := range assets {
		assetMap := asset.(map[string]interface{})
		info := assetMap["info"].(map[string]interface{})
		nativeToken := info["native_token"].(map[string]interface{})
		denom := nativeToken["denom"].(string)
		amount, _ := strconv.ParseInt(assetMap["amount"].(string), 10, 64)

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

func (p AstroportPosition) ComputeAddressPrincipalHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	if p.venuePositionConfig.ActiveShares == 0 {
		return &Holdings{
			Balances:  []Asset{},
			TotalUSDC: 0,
			TotalAtom: 0,
		}, nil
	}

	// Check what share of the pool the LP amounts correspond to
	withdrawQuery := map[string]interface{}{
		"share": map[string]interface{}{
			"amount": strconv.FormatInt(p.venuePositionConfig.ActiveShares, 10),
		},
	}

	withdrawData, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl,
		p.venuePositionConfig.PoolAddress, withdrawQuery)
	if err != nil {
		return nil, fmt.Errorf("simulating withdrawal: %s", err)
	}

	assets := withdrawData.([]interface{})
	var holdingAssets []Asset
	totalValueUSD := 0.0
	totalValueATOM := 0.0

	for _, asset := range assets {
		assetMap := asset.(map[string]interface{})
		info := assetMap["info"].(map[string]interface{})
		nativeToken := info["native_token"].(map[string]interface{})
		denom := nativeToken["denom"].(string)
		amount, _ := strconv.ParseInt(assetMap["amount"].(string), 10, 64)

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

func GetLPToken(p AstroportPosition) (string, error) {
	pairQuery := map[string]interface{}{
		"pair": map[string]interface{}{},
	}

	pairData, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl,
		p.venuePositionConfig.PoolAddress, pairQuery)
	if err != nil {
		return "", fmt.Errorf("querying pair info: %s", err)
	}

	lpToken := (pairData.(map[string]interface{}))["liquidity_token"].(string)
	return lpToken, nil
}

// We can only calculate rewards per address, not per bid.
func (p AstroportPosition) ComputeAddressRewardHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	if p.venuePositionConfig.ActiveShares == 0 {
		return &Holdings{
			Balances:  []Asset{},
			TotalUSDC: 0,
			TotalAtom: 0,
		}, nil
	}

	// First get LP token info
	lpToken, err := GetLPToken(p)
	if err != nil {
		return nil, err
	}

	rewardsQuery := map[string]interface{}{
		"pending_rewards": map[string]interface{}{
			"user":     address,
			"lp_token": lpToken,
		},
	}

	rewardsData, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl,
		p.venuePositionConfig.IncentiveAddress, rewardsQuery)
	if err != nil {
		// Check if error is "user doesn't have position"
		if strings.Contains(err.Error(), "doesn't have position") {
			return &Holdings{
				Balances:  []Asset{},
				TotalUSDC: 0,
				TotalAtom: 0,
			}, nil
		}
		return nil, fmt.Errorf("querying rewards: %s", err)
	}

	rewards := rewardsData.([]interface{})
	var rewardAssets []Asset
	totalValueUSD := 0.0
	totalValueATOM := 0.0

	for _, reward := range rewards {
		rewardMap := reward.(map[string]interface{})
		info := rewardMap["info"].(map[string]interface{})
		nativeToken := info["native_token"].(map[string]interface{})
		denom := nativeToken["denom"].(string)
		amount, _ := strconv.ParseInt(rewardMap["amount"].(string), 10, 64)

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

		rewardAssets = append(rewardAssets, Asset{
			Denom:       denom,
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
