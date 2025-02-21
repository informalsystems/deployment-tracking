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

type OsmosisBidPositionConfig struct {
	PoolID     string
	Address    string
	PositionID string
}

func (bidConfig OsmosisBidPositionConfig) GetProtocol() Protocol {
	return Osmosis
}

func (bidConfig OsmosisBidPositionConfig) GetPoolID() string {
	return bidConfig.PoolID
}

func (bidConfig OsmosisBidPositionConfig) GetAddress() string {
	return bidConfig.Address
}

// Osmosis implementation
type OsmosisPosition struct {
	protocolConfig    ProtocolConfig
	bidPositionConfig OsmosisBidPositionConfig
}

func NewOsmosisPosition(config ProtocolConfig, bidPositionConfig BidPositionConfig) (*OsmosisPosition, error) {
	osmosisBidPositionConfig, ok := bidPositionConfig.(OsmosisBidPositionConfig)
	if !ok {
		return nil, fmt.Errorf("bidPositionConfig must be of OsmosisBidPositionConfig type")
	}

	return &OsmosisPosition{protocolConfig: config, bidPositionConfig: osmosisBidPositionConfig}, nil
}

func (p OsmosisPosition) FetchPoolData() (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/pools?IDs=%s", p.protocolConfig.PoolInfoUrl, p.bidPositionConfig.PoolID)
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

func (p OsmosisPosition) ComputeTVL(assetData *ChainInfo) (*Holdings, error) {
	// Fetch pool data
	poolData, err := p.FetchPoolData()
	if err != nil {
		return nil, fmt.Errorf("fetching pool data: %s", err)
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
		tokenInfo := assetData.Tokens[denom]

		// Calculate adjusted amount
		adjustedAmount := float64(rawAmount) / math.Pow(10, float64(tokenInfo.Decimals))

		// Get token price from asset data
		usdValue := 0.0
		price, err := getTokenPrice(tokenInfo.CoingeckoID)
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
			DisplayName: tokenInfo.Display,
		}
		poolAssets = append(poolAssets, asset)
	}

	// Get ATOM price and calculate equivalent
	atomPrice, err := getAtomPrice()
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

func (p OsmosisPosition) fetchPositionsData(address string) (map[string]interface{}, error) {
	positionsURL := fmt.Sprintf("%s/osmosis/concentratedliquidity/v1beta1/positions/%s",
		p.protocolConfig.AddressBalanceUrl, address)

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

func (p *OsmosisPosition) calculateAssetValues(amounts map[string]int64, assetData *ChainInfo) ([]Asset, float64, error) {
	var assets []Asset
	totalUSD := 0.0

	for denom, amount := range amounts {
		tokenInfo := assetData.Tokens[denom]
		exp := tokenInfo.Decimals
		adjustedAmount := float64(amount) / math.Pow(10, float64(exp))
		displayName := tokenInfo.Display

		price, err := getTokenPrice(tokenInfo.CoingeckoID)
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
			DisplayName: displayName,
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

func (p OsmosisPosition) processPositionBalances(positions []interface{}) (map[string]int64, error) {
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

		// Only process the position that matches our position ID
		if posInfo["position_id"].(string) != p.bidPositionConfig.PositionID {
			continue
		}

		// check that the pool id matches what we expect for the position
		if poolID, ok := posInfo["pool_id"].(string); !ok || poolID != p.bidPositionConfig.PoolID {
			// return an error
			return nil, fmt.Errorf("pool ID mismatch: found %s for position %s, but expected %s", poolID, posInfo["position_id"].(string), p.bidPositionConfig.PoolID)
		}

		assets := []map[string]interface{}{
			position["asset0"].(map[string]interface{}),
			position["asset1"].(map[string]interface{}),
		}

		for _, asset := range assets {
			denom := asset["denom"].(string)
			amount, _ := strconv.ParseInt(asset["amount"].(string), 10, 64)
			balances[denom] = amount
		}

		// We found our position, no need to continue
		break
	}

	return balances, nil
}

func (p OsmosisPosition) processPositionRewards(positions []interface{}) (map[string]int64, error) {
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

		// Only process the position that matches our position ID
		if posInfo["position_id"].(string) != p.bidPositionConfig.PositionID {
			continue
		}

		// check that the pool id matches what we expect for the position
		if poolID, ok := posInfo["pool_id"].(string); !ok || poolID != p.bidPositionConfig.PoolID {
			// return an error
			return nil, fmt.Errorf("pool ID mismatch: found %s for position %s, but bid config claims %s", poolID, posInfo["position_id"].(string), p.bidPositionConfig.PoolID)
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

		// We found our position, no need to continue
		break
	}

	return rewards, nil
}

func (p OsmosisPosition) ComputeAddressPrincipalHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	positionsData, err := p.fetchPositionsData(address)
	if err != nil {
		return nil, err
	}

	positions, ok := positionsData["positions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid positions data structure")
	}

	balances, err := p.processPositionBalances(positions)
	if err != nil {
		return nil, err
	}

	assets, totalUSD, err := p.calculateAssetValues(balances, assetData)
	if err != nil {
		return nil, err
	}

	atomPrice, err := getAtomPrice()
	if err != nil {
		return nil, fmt.Errorf("getting ATOM price: %v", err)
	}

	return createHoldings(assets, totalUSD, atomPrice), nil
}

func (p OsmosisPosition) ComputeAddressRewardHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	positionsData, err := p.fetchPositionsData(address)
	if err != nil {
		return nil, err
	}

	positions, ok := positionsData["positions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid positions data structure")
	}

	rewards, err := p.processPositionRewards(positions)
	if err != nil {
		return nil, err
	}

	assets, totalUSD, err := p.calculateAssetValues(rewards, assetData)
	if err != nil {
		return nil, err
	}

	atomPrice, err := getAtomPrice()
	if err != nil {
		return nil, fmt.Errorf("getting ATOM price: %v", err)
	}

	return createHoldings(assets, totalUSD, atomPrice), nil
}
