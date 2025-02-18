package main

import (
	"encoding/json"
	"fmt"
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
}

func getTokenPrice(assetData map[string]interface{}, tokenDisplayName string) (float64, error) {
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

func buildTokenMapping(assetData map[string]interface{}) (*TokenMapping, error) {
	mapping := &TokenMapping{
		ExponentMap:    make(map[string]int),
		DisplayNameMap: make(map[string]string),
	}

	chain, ok := assetData["chain"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid asset data structure")
	}

	assets, ok := chain["assets"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid assets structure")
	}

	for _, asset := range assets {
		assetMap := asset.(map[string]interface{})
		denom := assetMap["denom"].(string)

		if decimals, ok := assetMap["decimals"].(float64); ok {
			mapping.ExponentMap[denom] = int(decimals)
		}

		if symbol, ok := assetMap["symbol"].(string); ok {
			mapping.DisplayNameMap[denom] = symbol
		}
	}

	return mapping, nil
}
