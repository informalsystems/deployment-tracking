package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

func getTokenValues(
	adjustedAmount float64,
	tokenInfo ChainTokenInfo,
) (float64, float64, error) {
	price, err := getTokenPrice(tokenInfo.CoingeckoID)
	if err != nil {
		return 0, 0, fmt.Errorf("fetching token price: %s", err)
	}

	usdValue := adjustedAmount * price
	atomPrice, err := getAtomPrice()
	if err != nil {
		return 0, 0, fmt.Errorf("fetching ATOM price: %s", err)
	}

	atomValue := usdValue / atomPrice

	return usdValue, atomValue, nil
}

type SkipAsset struct {
	Denom             string `json:"denom"`
	ChainID           string `json:"chain_id"`
	Symbol            string `json:"symbol"`
	Decimals          int    `json:"decimals"`
	CoingeckoID       string `json:"coingecko_id"`
	RecommendedSymbol string `json:"recommended_symbol"`
}

type SkipChainAssets struct {
	Assets []SkipAsset `json:"assets"`
}

type SkipResponse struct {
	ChainToAssetsMap map[string]SkipChainAssets `json:"chain_to_assets_map"`
}

// Global price cache
var (
	pricesInitialized bool = false
	priceCache        *PriceCache
	skipCache         *SkipCache
)

const PriceCacheTTL = 30 * time.Minute

type SkipCache struct {
	Assets    map[string]map[string]SkipAsset
	Timestamp time.Time
}

type PriceCache struct {
	Prices    map[string]float64
	Timestamp time.Time
}

// Fetch all prices in one call
func initializePriceCache() error {
	if pricesInitialized {
		if time.Since(priceCache.Timestamp) < PriceCacheTTL {
			return nil
		}
	}

	// refresh skip assets
	fetchSkipAssets()

	coinIDs := make(map[string]bool)
	for _, chainAssets := range skipCache.Assets {
		for _, asset := range chainAssets {
			if asset.CoingeckoID != "" {
				coinIDs[asset.CoingeckoID] = true
			}
		}
	}

	// Convert to comma-separated list
	var idList []string
	for id := range coinIDs {
		idList = append(idList, id)
	}

	// Batch fetch all prices
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd",
		strings.Join(idList, ","))

	debugLog("Fetching all CoinGecko prices", map[string]interface{}{
		"url":        url,
		"coin_count": len(idList),
	})

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetching coingecko prices: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding coingecko response: %v", err)
	}

	// Cache all prices
	prices := make(map[string]float64)
	now := time.Now()
	for coinID, priceData := range result {
		if usdPrice, ok := priceData["usd"]; ok {
			prices[coinID] = usdPrice
		}
	}

	priceCache = &PriceCache{
		Prices:    prices,
		Timestamp: now,
	}

	pricesInitialized = true
	debugLog("Price cache initialized", map[string]interface{}{
		"prices_cached": len(priceCache.Prices),
		"timestamp":     now,
	})
	return nil
}

func fetchSkipAssets() error {
	// Check if cache is still valid
	if skipCache != nil {
		if time.Since(skipCache.Timestamp) < PriceCacheTTL {
			return nil
		}
	}

	resp, err := http.Get("https://api.skip.build/v2/fungible/assets")
	if err != nil {
		return fmt.Errorf("fetching skip assets: %v", err)
	}
	defer resp.Body.Close()

	var skipResp SkipResponse
	if err := json.NewDecoder(resp.Body).Decode(&skipResp); err != nil {
		return fmt.Errorf("decoding skip response: %v", err)
	}

	// Create new cache
	assets := make(map[string]map[string]SkipAsset)
	for chainID, chainAssets := range skipResp.ChainToAssetsMap {
		assets[chainID] = make(map[string]SkipAsset)
		for _, asset := range chainAssets.Assets {
			assets[chainID][asset.Denom] = asset
		}
	}

	skipCache = &SkipCache{
		Assets:    assets,
		Timestamp: time.Now(),
	}

	return nil
}

func getTokenPrice(coingeckoId string) (float64, error) {
	debugLog("Getting token price", map[string]string{
		"token": coingeckoId,
	})

	// initialize the price cache (will be a no-op if the cache was already initialized
	// and not expired yet)
	if err := initializePriceCache(); err != nil {
		return 0, fmt.Errorf("refreshing price cache: %v", err)
	}

	// Try cache again after refresh
	if price, ok := priceCache.Prices[coingeckoId]; ok {
		return price, nil
	}

	return 0, fmt.Errorf("no price found for token: %s", coingeckoId)
}

func getAtomPrice() (float64, error) {
	return getTokenPrice("cosmos")
}

// Numia API types and constants
const (
	NumiaAPIBaseURL = "https://osmosis.numia.xyz/tokens/v2"
)

var NumiaAuthToken = os.Getenv("NUMIA_API_TOKEN")

type NumiaHistoricalPrice struct {
	Time   int64   `json:"time"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Open   float64 `json:"open"`
	Volume float64 `json:"volume"`
}

type NumiaRealtimePrice struct {
	Denom    string  `json:"denom"`
	USDPrice float64 `json:"usd_price"`
}

func getNumiaPrice(denom string) (float64, error) {
	// Replace standard IBC slash with percent encoded value
	encodedDenom := strings.Replace(denom, "ibc/", "ibc%2F", 1)
	url := fmt.Sprintf("%s/real-time/%s/price", NumiaAPIBaseURL, encodedDenom)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("creating request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", NumiaAuthToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetching price data: %v", err)
	}
	defer resp.Body.Close()

	var result NumiaRealtimePrice
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decoding price response: %v", err)
	}

	return result.USDPrice, nil
}

func getNumiaHistoricalPrice(denom string, timestamp int64) (float64, error) {
	// Replace standard IBC slash with percent encoded value
	encodedDenom := strings.Replace(denom, "ibc/", "ibc%2F", 1)
	url := fmt.Sprintf("%s/historical/%s/chart", NumiaAPIBaseURL, encodedDenom)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("creating request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", NumiaAuthToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetching historical price data: %v", err)
	}
	defer resp.Body.Close()

	var prices []NumiaHistoricalPrice
	if err := json.NewDecoder(resp.Body).Decode(&prices); err != nil {
		return 0, fmt.Errorf("decoding historical price response: %v", err)
	}

	// Find the closest price point to the requested timestamp
	var closestPrice *NumiaHistoricalPrice
	var smallestDiff int64 = math.MaxInt64

	for i := range prices {
		diff := abs64(prices[i].Time - timestamp)
		if diff < smallestDiff {
			smallestDiff = diff
			closestPrice = &prices[i]
		}
	}

	if closestPrice == nil {
		return 0, fmt.Errorf("no historical price data found for timestamp %d", timestamp)
	}

	return closestPrice.Close, nil
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func ComputeInitialHoldingsWithPrices(holdings *Holdings, assetData *ChainInfo, timestamp int64) (*Holdings, error) {
	var assets []Asset
	totalUSD := 0.0
	totalAtom := 0.0

	// Get ATOM price for conversion
	atomPrice, err := getNumiaHistoricalPrice("ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2", timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical ATOM price: %v", err)
	}

	for _, asset := range holdings.Balances {
		_, ok := assetData.Tokens[asset.Denom]
		if !ok {
			continue
		}

		// Get historical price from Numia API
		price, err := getNumiaHistoricalPrice(asset.Denom, timestamp)
		if err != nil {
			debugLog("Failed to get historical price, skipping asset", map[string]interface{}{
				"denom": asset.Denom,
				"error": err.Error(),
			})
			continue
		}

		usdValue := asset.Amount * price
		atomValue := usdValue / atomPrice

		totalUSD += usdValue
		totalAtom += atomValue

		assets = append(assets, Asset{
			Denom:       asset.Denom,
			Amount:      asset.Amount,
			DisplayName: asset.DisplayName,
			USDValue:    usdValue,
		})
	}

	return &Holdings{
		Balances:  assets,
		TotalUSDC: totalUSD,
		TotalAtom: totalAtom,
	}, nil
}

func init() {
	if NumiaAuthToken == "" {
		log.Fatal("NUMIA_API_TOKEN environment variable must be set")
	}

	if err := initializePriceCache(); err != nil {
		log.Printf("Warning: Failed to fetch Skip assets: %v", err)
	}
}
