package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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

	// supplement with the skip token list
	err = fetchSkipAssets()
	if err != nil {
		// if the skip assets couldn't be fetched, log an error, but continue
		debugLog("Failed to fetch skip assets", map[string]string{"error": err.Error()})
	}

	skipAssets := skipCache.Assets[chainID]
	for denom, asset := range skipAssets {
		debugLog("Adding skip asset", map[string]string{"denom": denom})
		if _, ok := tokens[denom]; !ok {
			tokens[denom] = ChainTokenInfo{
				Denom:       denom,
				Display:     asset.RecommendedSymbol,
				Decimals:    asset.Decimals,
				CoingeckoID: asset.CoingeckoID,
			}
		}
	}

	return &ChainInfo{
		ChainID: chainID,
		Tokens:  tokens,
	}, nil
}

// A type to parse error responses
type WasmError struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details"`
}

func QuerySmartContractData(nodeUrl string, contractAddress string,
	query map[string]interface{},
) (interface{}, error) {
	debugLog("Querying smart contract data", query)
	queryJson, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query into JSON: %s", err.Error())
	}

	queryEncoded := base64.StdEncoding.EncodeToString([]byte(queryJson))
	url := fmt.Sprintf("%s/%s/smart/%s",
		nodeUrl, contractAddress, string(queryEncoded))
	debugLog("Fetching data from smart contract", map[string]string{"url": url})

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching data failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		debugLog("Failed to fetch smart contract data", map[string]interface{}{
			"status_code": resp.StatusCode,
			"response":    string(body),
		})

		// Try to parse error message
		var wasmErr WasmError
		if err := json.Unmarshal(body, &wasmErr); err == nil {
			return nil, fmt.Errorf("error fetching smart contract data - wasm error response: %v", wasmErr.Message)
		}
		return nil, fmt.Errorf("fetching smart contract data: %d", resp.StatusCode)
	}

	var response struct {
		Data interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decoding smart contract data: %v", err)
	}

	debugLog("contract response", response)

	if response.Data == nil {
		return nil, fmt.Errorf("smart contract returned no data")
	}

	return response.Data, nil
}

func getJSON(url string, target interface{}) error {
	debugLog("Fetching JSON data", map[string]string{"url": url})

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("making HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		debugLog("Failed to fetch JSON data", map[string]interface{}{
			"status_code": resp.StatusCode,
			"response":    string(body),
		})
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}

	debugLog("Received JSON response", map[string]string{
		"body": string(body),
	})

	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decoding JSON response: %v", err)
	}

	return nil
}
