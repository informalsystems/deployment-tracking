package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type AstroportBidPositionConfig struct {
	PoolAddress      string // Contract address of the pool
	Address          string
	IncentiveAddress string
	ChainID          string
}

func (bidConfig AstroportBidPositionConfig) GetProtocol() Protocol {
	if bidConfig.ChainID == "neutron-1" {
		return AstroportNeutron
	} else if bidConfig.ChainID == "phoenix-1" {
		return AstroportTerra
	} else {
		debugLog("Unknown chain ID in bid config, defaulting to Astroport on Neutron", map[string]string{"chain_id": bidConfig.ChainID})
		return AstroportNeutron
	}
}

func (bidConfig AstroportBidPositionConfig) GetPoolID() string {
	return bidConfig.PoolAddress
}

func (bidConfig AstroportBidPositionConfig) GetAddress() string {
	return bidConfig.Address
}

type AstroportPosition struct {
	protocolConfig    ProtocolConfig
	bidPositionConfig AstroportBidPositionConfig
}

func NewAstroportPosition(config ProtocolConfig, bidPositionConfig BidPositionConfig) (*AstroportPosition, error) {
	astroportBidPositionConfig, ok := bidPositionConfig.(AstroportBidPositionConfig)
	if !ok {
		return nil, fmt.Errorf("bidPositionConfig must be of AstroportBidPositionConfig type")
	}

	return &AstroportPosition{
		protocolConfig:    config,
		bidPositionConfig: astroportBidPositionConfig,
	}, nil
}

func (p AstroportPosition) ComputeTVL(assetData *ChainInfo) (*Holdings, error) {
	// Query pool info
	queryMsg := map[string]interface{}{
		"pool": map[string]interface{}{},
	}

	data, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl,
		p.bidPositionConfig.PoolAddress, queryMsg)
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

	if err != nil {
		return nil, fmt.Errorf("getting ATOM price: %s", err)
	}

	return &Holdings{
		Balances:  poolAssets,
		TotalUSDC: totalValueUSD,
		TotalAtom: totalValueATOM,
	}, nil
}

func (p AstroportPosition) ComputeAddressPrincipalHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	// First get LP token info
	lpToken, err := GetLPToken(p)
	if err != nil {
		return nil, err
	}

	// Get user's LP balance and staked amount
	totalLPAmount, err := p.getTotalLPAmount(address, lpToken)
	if err != nil {
		return nil, fmt.Errorf("getting total LP amount: %s", err)
	}

	// Check what share of the pool the LP amounts correspond to
	withdrawQuery := map[string]interface{}{
		"share": map[string]interface{}{
			"amount": strconv.FormatInt(totalLPAmount, 10),
		},
	}

	withdrawData, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl,
		p.bidPositionConfig.PoolAddress, withdrawQuery)
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
		p.bidPositionConfig.PoolAddress, pairQuery)
	if err != nil {
		return "", fmt.Errorf("querying pair info: %s", err)
	}

	lpToken := (pairData.(map[string]interface{}))["liquidity_token"].(string)
	return lpToken, nil
}

func (p AstroportPosition) ComputeAddressRewardHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
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
		p.bidPositionConfig.IncentiveAddress, rewardsQuery)
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

func (p AstroportPosition) getTotalLPAmount(address string, lpToken string) (int64, error) {
	walletBalance := int64(0)

	// First try native token query
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

	// If not found in native tokens, try as CW20
	if walletBalance == 0 {
		balanceQuery := map[string]interface{}{
			"balance": map[string]interface{}{
				"address": address,
			},
		}

		balanceData, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl,
			lpToken, balanceQuery)
		if err == nil { // Only process if query succeeds
			balanceResponse, ok := balanceData.(map[string]interface{})
			if ok {
				if balanceStr, ok := balanceResponse["balance"].(string); ok {
					walletBalance, _ = strconv.ParseInt(balanceStr, 10, 64)
				}
			}
		}
		// Ignore errors as the token might not be a CW20 either
	}

	// Query staked balance
	stakedQuery := map[string]interface{}{
		"deposit": map[string]interface{}{
			"lp_token": lpToken,
			"user":     address,
		},
	}

	stakedData, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl,
		p.bidPositionConfig.IncentiveAddress, stakedQuery)
	if err != nil {
		return 0, fmt.Errorf("querying staked balance: %s", err)
	}

	stakedBalance, _ := strconv.ParseInt(stakedData.(string), 10, 64)

	return walletBalance + stakedBalance, nil
}
