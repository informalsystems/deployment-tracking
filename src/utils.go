package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// Helper functions
func debugLog(message string, data interface{}) {
	if Debug {
		fmt.Printf("[DEBUG] %s\n", message)
		if data != nil {
			jsonData, _ := json.MarshalIndent(data, "", "  ")
			fmt.Println(string(jsonData))
		}
	}
}

func fetchAssetList(assetListUrl string) (map[string]interface{}, error) {
	debugLog("Fetching asset list", map[string]string{"url": assetListUrl})

	resp, err := http.Get(assetListUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// TokenMapping holds token metadata
type TokenMapping struct {
	ExponentMap    map[string]int
	DisplayNameMap map[string]string
	CoingeckoIDMap map[string]string
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
	priceCache             = make(map[string]float64)
	skipAssetCache         = make(map[string]map[string]SkipAsset) // chainID -> denom -> asset
)

// Fetch all prices in one call
func initializePriceCache() error {
	if pricesInitialized {
		return nil
	}

	// Collect unique CoinGecko IDs
	coinIDs := make(map[string]bool)
	for _, chainAssets := range skipAssetCache {
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
	for coinID, priceData := range result {
		if usdPrice, ok := priceData["usd"]; ok {
			priceCache[coinID] = usdPrice
		}
	}

	pricesInitialized = true
	debugLog("Price cache initialized", map[string]interface{}{
		"prices_cached": len(priceCache),
	})
	return nil
}

func fetchSkipAssets() error {
	resp, err := http.Get("https://api.skip.build/v2/fungible/assets")
	if err != nil {
		return fmt.Errorf("fetching skip assets: %v", err)
	}
	defer resp.Body.Close()

	var skipResp SkipResponse
	if err := json.NewDecoder(resp.Body).Decode(&skipResp); err != nil {
		return fmt.Errorf("decoding skip response: %v", err)
	}

	// Cache assets by chain and denom
	for chainID, chainAssets := range skipResp.ChainToAssetsMap {
		skipAssetCache[chainID] = make(map[string]SkipAsset)
		for _, asset := range chainAssets.Assets {
			skipAssetCache[chainID][asset.Denom] = asset
		}
	}

	// Initialize price cache
	if err := initializePriceCache(); err != nil {
		return fmt.Errorf("initializing price cache: %v", err)
	}

	return nil
}

// Fetch CoinGecko price
func fetchCoinGeckoPrice(coinID string) (float64, error) {
	// Check cache first
	if price, ok := priceCache[coinID]; ok {
		return price, nil
	}

	url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd", coinID)
	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("fetching coingecko price: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decoding coingecko response: %v", err)
	}

	if price, ok := result[coinID]["usd"]; ok {
		priceCache[coinID] = price
		return price, nil
	}

	return 0, fmt.Errorf("no price found for %s", coinID)
}

func buildTokenMapping(assetData map[string]interface{}, chainID string) (*TokenMapping, error) {
	mapping := &TokenMapping{
		ExponentMap:    make(map[string]int),
		DisplayNameMap: make(map[string]string),
		CoingeckoIDMap: make(map[string]string),
	}

	// Try primary asset data first
	chain, ok := assetData["chain"].(map[string]interface{})
	if ok {
		if assets, ok := chain["assets"].([]interface{}); ok {
			for _, asset := range assets {
				assetMap := asset.(map[string]interface{})
				denom := assetMap["denom"].(string)

				if decimals, ok := assetMap["decimals"].(float64); ok {
					mapping.ExponentMap[denom] = int(decimals)
				}
				if symbol, ok := assetMap["symbol"].(string); ok {
					mapping.DisplayNameMap[denom] = symbol
				}
				if coingeckoID, ok := assetMap["coingecko_id"].(string); ok {
					mapping.CoingeckoIDMap[denom] = coingeckoID
				}
			}
		}
	}

	// Fill in missing data from Skip assets
	if skipAssets, ok := skipAssetCache[chainID]; ok {
		for denom, asset := range skipAssets {
			if _, ok := mapping.ExponentMap[denom]; !ok {
				mapping.ExponentMap[denom] = asset.Decimals
			}
			if _, ok := mapping.DisplayNameMap[denom]; !ok {
				mapping.DisplayNameMap[denom] = asset.RecommendedSymbol
			}
			if _, ok := mapping.CoingeckoIDMap[denom]; !ok {
				mapping.CoingeckoIDMap[denom] = asset.CoingeckoID
			}
		}
	}

	return mapping, nil
}

// Tries to get the price from the chain directory
func getChainDirectoryPrice(assetData map[string]interface{}, tokenDisplayName string) (float64, error) {
	debugLog("Getting token price", map[string]string{"token": tokenDisplayName})

	// Navigate to coingecko prices
	chain, ok := assetData["chain"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid chain data structure")
	}

	prices, ok := chain["prices"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid prices data structure")
	}

	coingeckoPrices, ok := prices["coingecko"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid coingecko prices data structure")
	}

	// Try exact match first
	if priceData, ok := coingeckoPrices[tokenDisplayName].(map[string]interface{}); ok {
		if price, ok := priceData["usd"].(float64); ok {
			debugLog("Found price for token", map[string]interface{}{
				"token": tokenDisplayName,
				"price": price,
			})
			return price, nil
		}
	}

	// Try lowercase version
	if priceData, ok := coingeckoPrices[strings.ToLower(tokenDisplayName)].(map[string]interface{}); ok {
		if price, ok := priceData["usd"].(float64); ok {
			debugLog("Found price for lowercase token", map[string]interface{}{
				"token": strings.ToLower(tokenDisplayName),
				"price": price,
			})
			return price, nil
		}
	}

	debugLog("No price found for token", map[string]string{"token": tokenDisplayName})
	return 0, fmt.Errorf("price not found for token: %s", tokenDisplayName)
}

func getTokenPrice(assetData map[string]interface{}, tokenDisplayName string, chainID string, denom string) (float64, error) {
	debugLog("Getting token price", map[string]string{
		"token": tokenDisplayName,
		"chain": chainID,
		"denom": denom,
	})

	// Try primary source first
	if price, err := getChainDirectoryPrice(assetData, tokenDisplayName); err == nil {
		return price, nil
	}

	// Try Skip assets cache
	if skipAssets, ok := skipAssetCache[chainID]; ok {
		if asset, ok := skipAssets[denom]; ok && asset.CoingeckoID != "" {
			if price, ok := priceCache[asset.CoingeckoID]; ok {
				return price, nil
			}
		}
	}

	return 0, fmt.Errorf("no price found for token: %s", tokenDisplayName)
}

// Initialize caches in main
func init() {
	if err := fetchSkipAssets(); err != nil {
		log.Printf("Warning: Failed to fetch Skip assets: %v", err)
	}
}
