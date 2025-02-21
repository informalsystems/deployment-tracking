package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	priceCache             = make(map[string]PriceInfo)
	skipCache         *SkipCache
)

const PriceCacheTTL = 30 * time.Minute

type PriceInfo struct {
	Price     float64
	Timestamp time.Time
}

type SkipCache struct {
	Assets    map[string]map[string]SkipAsset
	Timestamp time.Time
}

// Fetch all prices in one call
func initializePriceCache() error {
	if pricesInitialized {
		for _, info := range priceCache {
			if time.Since(info.Timestamp) < PriceCacheTTL {
				return nil
			}
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
	now := time.Now()
	for coinID, priceData := range result {
		if usdPrice, ok := priceData["usd"]; ok {
			priceCache[coinID] = PriceInfo{
				Price:     usdPrice,
				Timestamp: now,
			}
		}
	}

	pricesInitialized = true
	debugLog("Price cache initialized", map[string]interface{}{
		"prices_cached": len(priceCache),
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

	// Initialize price cache
	if err := initializePriceCache(); err != nil {
		return fmt.Errorf("initializing price cache: %v", err)
	}

	return nil
}

func getTokenPrice(coingeckoId string) (float64, error) {
	debugLog("Getting token price", map[string]string{
		"token": coingeckoId,
	})

	// Check cache
	if info, ok := priceCache[coingeckoId]; ok {
		if time.Since(info.Timestamp) < PriceCacheTTL {
			return info.Price, nil
		}
	}

	// Cache expired or missing, refresh all prices
	if err := initializePriceCache(); err != nil {
		return 0, fmt.Errorf("refreshing price cache: %v", err)
	}

	// Try cache again after refresh
	if info, ok := priceCache[coingeckoId]; ok {
		return info.Price, nil
	}

	return 0, fmt.Errorf("no price found for token: %s", coingeckoId)
}

func getAtomPrice() (float64, error) {
	return getTokenPrice("cosmos")
}

// Initialize caches in main
func init() {
	if err := fetchSkipAssets(); err != nil {
		log.Printf("Warning: Failed to fetch Skip assets: %v", err)
	}
}
