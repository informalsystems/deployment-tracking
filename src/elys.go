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

type ElysVenuePositionConfig struct {
	PoolId  string
	Address string
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

func (p ElysPosition) fetchPoolData() (map[string]interface{}, error) {
	poolURL := fmt.Sprintf("%s/stablestake/pool/%s", p.protocolConfig.PoolInfoUrl, p.venuePositionConfig.PoolId)

	resp, err := http.Get(poolURL)
	if err != nil {
		return nil, fmt.Errorf("fetching pool info: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching pool info: status %d", resp.StatusCode)
	}

	var poolData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&poolData); err != nil {
		return nil, fmt.Errorf("decoding pool response: %v", err)
	}

	return poolData, nil
}

func (p ElysPosition) ComputeTVL(assetData *ChainInfo) (*Holdings, error) {
	poolData, err := p.fetchPoolData()
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

func (p ElysPosition) ComputeAddressPrincipalHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	commitmentURL := fmt.Sprintf("%s/commitment/committed_tokens_locked/%s", p.protocolConfig.PoolInfoUrl, address)

	resp, err := http.Get(commitmentURL)
	if err != nil {
		return nil, fmt.Errorf("fetching commitment data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching commitment data: status %d", resp.StatusCode)
	}

	var commitmentData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&commitmentData); err != nil {
		return nil, fmt.Errorf("decoding commitment response: %v", err)
	}

	totalCommitted, ok := commitmentData["total_committed"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid total_committed in commitment data")
	}

	poolId, err := strconv.ParseUint(p.venuePositionConfig.PoolId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing PoolId: %v", err)
	}

	shareDenom := GetShareDenomForPool(poolId)

	var amount int64
	for _, committed := range totalCommitted {
		committedMap, ok := committed.(map[string]interface{})
		if !ok {
			continue
		}

		denom, ok := committedMap["denom"].(string)
		if !ok || denom != shareDenom {
			continue
		}

		amountStr, ok := committedMap["amount"].(string)
		if !ok {
			continue
		}

		amount, err = strconv.ParseInt(amountStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing amount: %v", err)
		}
		break
	}

	if amount == 0 {
		return nil, fmt.Errorf("no matching denom found in commitment data")
	}

	var poolData map[string]interface{}
	if p.cachedPoolData == nil {
		p.cachedPoolData, err = p.fetchPoolData()
		if err != nil {
			return nil, err
		}
	}
	poolData = p.cachedPoolData

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
			Denom:       shareDenom,
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

func (p ElysPosition) ComputeAddressRewardHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
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

func GetShareDenomForPool(poolId uint64) string {
	if poolId == UsdcPoolId {
		return "stablestake/share"
	}
	return "stablestake/share/pool/" + strconv.FormatUint(poolId, 10)
}
