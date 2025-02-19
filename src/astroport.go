package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
)

// Astroport implementation
type AstroportPosition struct {
	protocolConfig    ProtocolConfig
	bidPositionConfig BidPositionConfig
}

func NewAstroportPosition(config ProtocolConfig, bidPositionConfig BidPositionConfig) AstroportPosition {
	return AstroportPosition{
		protocolConfig:    config,
		bidPositionConfig: bidPositionConfig,
	}
}

// Pool response structure matching Astroport contract
type PoolResponse struct {
	Assets []struct {
		Info struct {
			NativeToken struct {
				Denom string `json:"denom"`
			} `json:"native_token"`
		} `json:"info"`
		Amount string `json:"amount"`
	} `json:"assets"`
}

func (p AstroportPosition) ComputeTVL(assetData map[string]interface{}) (*Holdings, error) {
	// Build pool query message
	queryMsg := map[string]interface{}{
		"pool": map[string]interface{}{},
	}

	// Convert to JSON and base64 encode
	queryJSON, err := json.Marshal(queryMsg)
	if err != nil {
		return nil, fmt.Errorf("marshaling query: %v", err)
	}
	queryB64 := base64.StdEncoding.EncodeToString(queryJSON)

	// Build query URL
	url := fmt.Sprintf("%s/cosmwasm/wasm/v1/contract/%s/smart/%s",
		p.protocolConfig.PoolInfoUrl,
		p.bidPositionConfig.PoolAddress,
		queryB64)

	log.Printf("Querying pool: %s", url)

	// Make request
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("querying pool: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad response: %d", resp.StatusCode)
	}

	// Parse response
	var result struct {
		Data PoolResponse `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %v", err)
	}

	// Get token mappings
	mapping, err := buildTokenMapping(assetData, p.protocolConfig.ChainID)
	if err != nil {
		return nil, fmt.Errorf("building token mapping: %v", err)
	}

	// Process assets
	var poolAssets []Asset
	totalValueUSD := 0.0

	log.Println("Processing assets: ", result.Data.Assets)

	for _, asset := range result.Data.Assets {
		denom := asset.Info.NativeToken.Denom
		amount, err := strconv.ParseInt(asset.Amount, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing amount: %v", err)
		}

		exp, ok := mapping.ExponentMap[denom]
		if !ok {
			return nil, fmt.Errorf("missing exponent for denom %s", denom)
		}
		displayName, ok := mapping.DisplayNameMap[denom]
		if !ok {
			return nil, fmt.Errorf("missing display name for denom %s", denom)
		}

		// Calculate adjusted amount
		adjustedAmount := float64(amount) / math.Pow(10, float64(exp))

		// Get token price
		price, err := getTokenPrice(assetData, displayName, p.protocolConfig.ChainID, coingeckoID)
		if err != nil {
			return nil, fmt.Errorf("getting token price: %v", err)
		}

		usdValue := adjustedAmount * price
		totalValueUSD += usdValue

		poolAssets = append(poolAssets, Asset{
			Denom:       denom,
			Amount:      adjustedAmount,
			USDValue:    usdValue,
			DisplayName: &displayName,
		})
	}

	// Get ATOM price for conversion
	atomPrice, err := getTokenPrice(assetData, "atom", p.protocolConfig.ChainID, "cosmos")
	if err != nil {
		return nil, fmt.Errorf("getting ATOM price: %v", err)
	}

	totalAtomValue := 0.0
	if atomPrice > 0 {
		totalAtomValue = totalValueUSD / atomPrice
	}

	return &Holdings{
		Balances:  poolAssets,
		TotalUSDC: totalValueUSD,
		TotalAtom: totalAtomValue,
	}, nil
}

func (p AstroportPosition) ComputeAddressPrincipalHoldings(assetData map[string]interface{}, address string) (*Holdings, error) {
	// 1. Query pair info to get LP token denom
	pairQuery := map[string]interface{}{
		"pair": map[string]interface{}{},
	}

	queryJSON, err := json.Marshal(pairQuery)
	if err != nil {
		return nil, fmt.Errorf("marshaling pair query: %v", err)
	}
	queryB64 := base64.StdEncoding.EncodeToString(queryJSON)

	pairURL := fmt.Sprintf("%s/cosmwasm/wasm/v1/contract/%s/smart/%s",
		p.protocolConfig.PoolInfoUrl,
		p.bidPositionConfig.PoolAddress,
		queryB64)

	resp, err := http.Get(pairURL)
	if err != nil {
		return nil, fmt.Errorf("querying pair info: %v", err)
	}
	defer resp.Body.Close()

	var pairResponse struct {
		Data struct {
			LiquidityToken string `json:"liquidity_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pairResponse); err != nil {
		return nil, fmt.Errorf("decoding pair response: %v", err)
	}

	// 2. Query bank balance for LP tokens
	balanceURL := fmt.Sprintf("%s/cosmos/bank/v1beta1/balances/%s",
		p.protocolConfig.PoolInfoUrl, address)

	resp, err = http.Get(balanceURL)
	if err != nil {
		return nil, fmt.Errorf("querying balance: %v", err)
	}
	defer resp.Body.Close()

	var balanceResponse struct {
		Balances []struct {
			Denom  string `json:"denom"`
			Amount string `json:"amount"`
		} `json:"balances"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&balanceResponse); err != nil {
		return nil, fmt.Errorf("decoding balance response: %v", err)
	}

	// Find LP token balance
	lpBalance := "0"
	for _, bal := range balanceResponse.Balances {
		if bal.Denom == pairResponse.Data.LiquidityToken {
			lpBalance = bal.Amount
			break
		}
	}

	// 3. Query incentives contract for staked amount
	incentiveQuery := map[string]interface{}{
		"deposit": map[string]interface{}{
			"lp_token": pairResponse.Data.LiquidityToken,
			"user":     address,
		},
	}

	queryJSON, err = json.Marshal(incentiveQuery)
	if err != nil {
		return nil, fmt.Errorf("marshaling incentive query: %v", err)
	}
	queryB64 = base64.StdEncoding.EncodeToString(queryJSON)

	incentiveURL := fmt.Sprintf("%s/cosmwasm/wasm/v1/contract/%s/smart/%s",
		p.protocolConfig.PoolInfoUrl,
		p.bidPositionConfig.IncentiveAddress,
		queryB64)

	resp, err = http.Get(incentiveURL)
	if err != nil {
		return nil, fmt.Errorf("querying incentives: %v", err)
	}
	defer resp.Body.Close()

	var incentiveResponse struct {
		Data string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&incentiveResponse); err != nil {
		return nil, fmt.Errorf("decoding incentive response: %v", err)
	}

	// 4. Calculate total LP amount and simulate withdraw
	userBalance, _ := strconv.ParseInt(lpBalance, 10, 64)
	stakedBalance, _ := strconv.ParseInt(incentiveResponse.Data, 10, 64)
	totalBalance := userBalance + stakedBalance

	withdrawQuery := map[string]interface{}{
		"simulate_withdraw": map[string]interface{}{
			"lp_amount": strconv.FormatInt(totalBalance, 10),
		},
	}

	queryJSON, err = json.Marshal(withdrawQuery)
	if err != nil {
		return nil, fmt.Errorf("marshaling withdraw query: %v", err)
	}
	queryB64 = base64.StdEncoding.EncodeToString(queryJSON)

	withdrawURL := fmt.Sprintf("%s/cosmwasm/wasm/v1/contract/%s/smart/%s",
		p.protocolConfig.PoolInfoUrl,
		p.bidPositionConfig.PoolAddress,
		queryB64)

	resp, err = http.Get(withdrawURL)
	if err != nil {
		return nil, fmt.Errorf("simulating withdraw: %v", err)
	}
	defer resp.Body.Close()

	var withdrawResponse struct {
		Data []struct {
			Info struct {
				NativeToken struct {
					Denom string `json:"denom"`
				} `json:"native_token"`
			} `json:"info"`
			Amount string `json:"amount"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&withdrawResponse); err != nil {
		return nil, fmt.Errorf("decoding withdraw response: %v", err)
	}

	// 5. Convert to Holdings
	mapping, err := buildTokenMapping(assetData, p.protocolConfig.ChainID)
	if err != nil {
		return nil, fmt.Errorf("building token mapping: %v", err)
	}

	var assets []Asset
	totalValueUSD := 0.0

	for _, coin := range withdrawResponse.Data {
		denom := coin.Info.NativeToken.Denom
		amount, _ := strconv.ParseInt(coin.Amount, 10, 64)
		exp := mapping.ExponentMap[denom]
		displayName := mapping.DisplayNameMap[denom]

		adjustedAmount := float64(amount) / math.Pow(10, float64(exp))

		price, err := getTokenPrice(assetData, displayName, p.protocolConfig.ChainID, coingeckoID)
		if err != nil {
			return nil, fmt.Errorf("getting token price: %v", err)
		}

		usdValue := adjustedAmount * price
		totalValueUSD += usdValue

		asset := Asset{
			Denom:       denom,
			Amount:      adjustedAmount,
			USDValue:    usdValue,
			DisplayName: &displayName,
		}
		assets = append(assets, asset)
	}

	atomPrice, err := getTokenPrice(assetData, "atom", p.protocolConfig.ChainID, "cosmos")
	if err != nil {
		return nil, fmt.Errorf("getting ATOM price: %v", err)
	}

	totalAtomValue := 0.0
	if atomPrice > 0 {
		totalAtomValue = totalValueUSD / atomPrice
	}

	return &Holdings{
		Balances:  assets,
		TotalUSDC: totalValueUSD,
		TotalAtom: totalAtomValue,
	}, nil
}

func (p AstroportPosition) ComputeAddressRewardHoldings(assetData map[string]interface{}, address string) (*Holdings, error) {
	// Query incentives for accrued rewards
	rewardsQuery := map[string]interface{}{
		"rewards": map[string]interface{}{
			"address": address,
		},
	}

	queryJSON, err := json.Marshal(rewardsQuery)
	if err != nil {
		return nil, fmt.Errorf("marshaling rewards query: %v", err)
	}
	queryB64 := base64.StdEncoding.EncodeToString(queryJSON)

	rewardsURL := fmt.Sprintf("%s/cosmwasm/wasm/v1/contract/%s/smart/%s",
		p.protocolConfig.PoolInfoUrl,
		p.bidPositionConfig.IncentiveAddress,
		queryB64)

	resp, err := http.Get(rewardsURL)
	if err != nil {
		return nil, fmt.Errorf("querying rewards: %v", err)
	}
	defer resp.Body.Close()

	var rewardsResponse struct {
		Data struct {
			Rewards []struct {
				Info struct {
					NativeToken struct {
						Denom string `json:"denom"`
					} `json:"native_token"`
				} `json:"info"`
				Amount string `json:"amount"`
			} `json:"rewards"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rewardsResponse); err != nil {
		return nil, fmt.Errorf("decoding rewards response: %v", err)
	}

	// Build token mappings
	mapping, err := buildTokenMapping(assetData, p.protocolConfig.ChainID)
	if err != nil {
		return nil, fmt.Errorf("building token mapping: %v", err)
	}

	// Process rewards
	var assets []Asset
	totalValueUSD := 0.0

	for _, reward := range rewardsResponse.Data.Rewards {
		denom := reward.Info.NativeToken.Denom
		amount, err := strconv.ParseInt(reward.Amount, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing reward amount: %v", err)
		}

		exp := mapping.ExponentMap[denom]
		displayName := mapping.DisplayNameMap[denom]
		adjustedAmount := float64(amount) / math.Pow(10, float64(exp))

		price, err := getTokenPrice(assetData, displayName, p.protocolConfig.ChainID, coingeckoID)
		if err != nil {
			return nil, fmt.Errorf("getting token price: %v", err)
		}

		usdValue := adjustedAmount * price
		totalValueUSD += usdValue

		asset := Asset{
			Denom:       denom,
			Amount:      adjustedAmount,
			USDValue:    usdValue,
			DisplayName: &displayName,
		}
		assets = append(assets, asset)
	}

	// Get ATOM price for conversion
	atomPrice, err := getTokenPrice(assetData, "atom", p.protocolConfig.ChainID, "cosmos")
	if err != nil {
		return nil, fmt.Errorf("getting ATOM price: %v", err)
	}

	totalAtomValue := 0.0
	if atomPrice > 0 {
		totalAtomValue = totalValueUSD / atomPrice
	}

	return &Holdings{
		Balances:  assets,
		TotalUSDC: totalValueUSD,
		TotalAtom: totalAtomValue,
	}, nil
}
