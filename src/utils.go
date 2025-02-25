package main

import (
	"encoding/json"
	"fmt"
	"net/http"
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

func fetchAssetList(assetListUrl string) (*ChainInfo, error) {
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

	chain, ok := result["chain"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid asset data structure")
	}

	chainID, ok := chain["chain_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing chain_id")
	}

	assets, ok := chain["assets"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid assets structure")
	}

	tokens := make(map[string]ChainTokenInfo)
	for _, asset := range assets {
		assetMap := asset.(map[string]interface{})

		denom, ok := assetMap["denom"].(string)
		if !ok {
			continue
		}

		token := ChainTokenInfo{
			Denom: denom,
		}

		if symbol, ok := assetMap["symbol"].(string); ok {
			token.Display = symbol
		}

		if decimals, ok := assetMap["decimals"].(float64); ok {
			token.Decimals = int(decimals)
		}

		if coingeckoID, ok := assetMap["coingecko_id"].(string); ok {
			token.CoingeckoID = coingeckoID
		}

		tokens[denom] = token
	}

	return &ChainInfo{
		ChainID: chainID,
		Tokens:  tokens,
	}, nil
}
