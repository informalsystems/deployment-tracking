package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
)

const OsmosisAPIURL = "https://sqs.osmosis.zone"

// Osmosis implementation
type OsmosisProtocol struct {
	config ProtocolConfig
}

func NewOsmosisProtocol(config ProtocolConfig) OsmosisProtocol {
	return OsmosisProtocol{config: config}
}

func (p *OsmosisProtocol) FetchPoolData() (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/pools?IDs=%s", OsmosisAPIURL, p.config.PoolID)
	debugLog("Fetching pool data from Osmosis API", map[string]string{"url": url})

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching pool data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		debugLog("Failed to fetch pool data", map[string]interface{}{
			"status_code": resp.StatusCode,
			"response":    string(body),
		})
		return nil, fmt.Errorf("fetching pool data: %d", resp.StatusCode)
	}

	var pools []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&pools); err != nil {
		return nil, fmt.Errorf("decoding pool data: %v", err)
	}

	if len(pools) == 0 {
		return nil, fmt.Errorf("no pool data returned")
	}

	debugLog("Received pool data", pools[0])
	return pools[0], nil
}

func (p *OsmosisProtocol) ComputeTVL(assetData map[string]interface{}) (*Holdings, error) {
	// Fetch pool data
	poolData, err := p.FetchPoolData()
	if err != nil {
		return nil, fmt.Errorf("fetching pool data: %s", err)
	}

	// Get token mappings
	mapping, err := buildTokenMapping(assetData)
	if err != nil {
		return nil, fmt.Errorf("building token mapping: %s", err)
	}

	// Get balances array from pool data
	balances, ok := poolData["balances"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid pool balances structure")
	}

	// Track individual asset information
	var poolAssets []Asset
	totalValueUSD := 0.0

	// Process each balance
	for _, balance := range balances {
		balanceMap, ok := balance.(map[string]interface{})
		if !ok {
			continue
		}

		denom := balanceMap["denom"].(string)
		rawAmount, _ := strconv.ParseInt(balanceMap["amount"].(string), 10, 64)
		exp := mapping.ExponentMap[denom]
		displayName := mapping.DisplayNameMap[denom]

		// Calculate adjusted amount
		adjustedAmount := float64(rawAmount) / math.Pow(10, float64(exp))

		// Get token price from asset data
		usdValue := 0.0
		price, err := getTokenPrice(assetData, displayName)
		if err != nil {
			return nil, fmt.Errorf("fetching token price: %s", err)
		}

		// Calculate USD value
		usdValue = adjustedAmount * price
		totalValueUSD += usdValue

		// Create Asset object
		asset := Asset{
			Denom:       denom,
			Amount:      adjustedAmount,
			CoingeckoID: nil, // Optional field
			USDValue:    usdValue,
			DisplayName: &displayName,
		}
		poolAssets = append(poolAssets, asset)
	}

	// Get ATOM price and calculate equivalent
	atomPrice, err := getTokenPrice(assetData, "atom")
	if err != nil {
		return nil, fmt.Errorf("fetching ATOM price: %s", err)
	}

	totalAtomValue := 0.0
	if atomPrice > 0 {
		totalAtomValue = totalValueUSD / atomPrice
	}

	// Return Holdings object
	return &Holdings{
		Balances:  poolAssets,
		TotalUSDC: totalValueUSD,
		TotalAtom: totalAtomValue,
	}, nil
}

func (p *OsmosisProtocol) fetchPositionsData(address string) (map[string]interface{}, error) {
	positionsURL := fmt.Sprintf("%s/osmosis/concentratedliquidity/v1beta1/positions/%s",
		p.config.LCDEndpoint, address)

	resp, err := http.Get(positionsURL)
	if err != nil {
		return nil, fmt.Errorf("fetching positions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching positions: status %d", resp.StatusCode)
	}

	var positionsData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&positionsData); err != nil {
		return nil, fmt.Errorf("decoding positions: %v", err)
	}

	return positionsData, nil
}

func (p *OsmosisProtocol) calculateAssetValues(amounts map[string]int64, mapping *TokenMapping, assetData map[string]interface{}) ([]Asset, float64, error) {
	var assets []Asset
	totalUSD := 0.0

	for denom, amount := range amounts {
		exp := mapping.ExponentMap[denom]
		adjustedAmount := float64(amount) / math.Pow(10, float64(exp))
		displayName := mapping.DisplayNameMap[denom]

		price, err := getTokenPrice(assetData, displayName)
		if err != nil {
			return nil, 0, fmt.Errorf("getting token price: %v", err)
		}

		usdValue := adjustedAmount * price
		totalUSD += usdValue

		asset := Asset{
			Denom:       denom,
			Amount:      adjustedAmount,
			CoingeckoID: nil,
			USDValue:    usdValue,
			DisplayName: &displayName,
		}
		assets = append(assets, asset)
	}

	return assets, totalUSD, nil
}

func createHoldings(assets []Asset, totalUSD float64, atomPrice float64) *Holdings {
	totalAtom := 0.0
	if atomPrice > 0 {
		totalAtom = totalUSD / atomPrice
	}

	return &Holdings{
		Balances:  assets,
		TotalUSDC: totalUSD,
		TotalAtom: totalAtom,
	}
}

func (p *OsmosisProtocol) processPositionBalances(positions []interface{}) (map[string]int64, error) {
	balances := make(map[string]int64)

	for _, pos := range positions {
		position, ok := pos.(map[string]interface{})
		if !ok {
			continue
		}

		posInfo, ok := position["position"].(map[string]interface{})
		if !ok {
			continue
		}

		if fmt.Sprint(posInfo["pool_id"]) != p.config.PoolID {
			continue
		}

		assets := []map[string]interface{}{
			position["asset0"].(map[string]interface{}),
			position["asset1"].(map[string]interface{}),
		}

		for _, asset := range assets {
			denom := asset["denom"].(string)
			amount, _ := strconv.ParseInt(asset["amount"].(string), 10, 64)
			balances[denom] += amount
		}
	}

	return balances, nil
}

func (p *OsmosisProtocol) processPositionRewards(positions []interface{}) (map[string]int64, error) {
	rewards := make(map[string]int64)

	for _, pos := range positions {
		position, ok := pos.(map[string]interface{})
		if !ok {
			continue
		}

		posInfo, ok := position["position"].(map[string]interface{})
		if !ok {
			continue
		}

		if fmt.Sprint(posInfo["pool_id"]) != p.config.PoolID {
			continue
		}

		if spreadRewards, ok := position["claimable_spread_rewards"].([]interface{}); ok {
			for _, reward := range spreadRewards {
				rewardMap := reward.(map[string]interface{})
				denom := rewardMap["denom"].(string)
				amount, _ := strconv.ParseInt(rewardMap["amount"].(string), 10, 64)
				rewards[denom] += amount
			}
		}

		if incentiveRewards, ok := position["claimable_incentives"].([]interface{}); ok {
			for _, reward := range incentiveRewards {
				rewardMap := reward.(map[string]interface{})
				denom := rewardMap["denom"].(string)
				amount, _ := strconv.ParseInt(rewardMap["amount"].(string), 10, 64)
				rewards[denom] += amount
			}
		}
	}

	return rewards, nil
}

func (p *OsmosisProtocol) ComputeAddressPrincipalHoldings(assetData map[string]interface{}, address string) (*Holdings, error) {
	positionsData, err := p.fetchPositionsData(address)
	if err != nil {
		return nil, err
	}

	mapping, err := buildTokenMapping(assetData)
	if err != nil {
		return nil, fmt.Errorf("building token mapping: %v", err)
	}

	positions, ok := positionsData["positions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid positions data structure")
	}

	balances, err := p.processPositionBalances(positions)
	if err != nil {
		return nil, err
	}

	assets, totalUSD, err := p.calculateAssetValues(balances, mapping, assetData)
	if err != nil {
		return nil, err
	}

	atomPrice, err := getTokenPrice(assetData, "atom")
	if err != nil {
		return nil, fmt.Errorf("getting ATOM price: %v", err)
	}

	return createHoldings(assets, totalUSD, atomPrice), nil
}

func (p *OsmosisProtocol) ComputeAddressRewardHoldings(assetData map[string]interface{}, address string) (*Holdings, error) {
	positionsData, err := p.fetchPositionsData(address)
	if err != nil {
		return nil, err
	}

	mapping, err := buildTokenMapping(assetData)
	if err != nil {
		return nil, fmt.Errorf("building token mapping: %v", err)
	}

	positions, ok := positionsData["positions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid positions data structure")
	}

	rewards, err := p.processPositionRewards(positions)
	if err != nil {
		return nil, err
	}

	assets, totalUSD, err := p.calculateAssetValues(rewards, mapping, assetData)
	if err != nil {
		return nil, err
	}

	atomPrice, err := getTokenPrice(assetData, "atom")
	if err != nil {
		return nil, fmt.Errorf("getting ATOM price: %v", err)
	}

	return createHoldings(assets, totalUSD, atomPrice), nil
}
