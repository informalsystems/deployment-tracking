package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
)

const UX_ATOM = "ibc/C4CFF46FD6DE35CA4CF4CE031E643C8FDC9BA4B99AE598E9B0ED98FE3A2319F9"

type UxVenuePositionConfig struct {
	Denom   string
	Address string
}

func (venueConfig UxVenuePositionConfig) GetProtocol() Protocol {
	return Ux
}

func (venueConfig UxVenuePositionConfig) GetPoolID() string {
	return ""
}

func (venueConfig UxVenuePositionConfig) GetAddress() string {
	return venueConfig.Address
}

type UxPosition struct {
	protocolConfig      ProtocolConfig
	venuePositionConfig UxVenuePositionConfig
}

func NewUxPosition(config ProtocolConfig, venuePositionConfig VenuePositionConfig) (*UxPosition, error) {
	UxVenuePositionConfig, ok := venuePositionConfig.(UxVenuePositionConfig)
	if !ok {
		return nil, fmt.Errorf("venuePositionConfig must be of UxVenuePositionConfig type")
	}

	return &UxPosition{
		protocolConfig:      config,
		venuePositionConfig: UxVenuePositionConfig,
	}, nil
}

func (p UxPosition) ComputeTVL(assetData *ChainInfo) (*Holdings, error) {
	// Fetch market summary
	marketSummary, err := p.getMarketSummary()
	if err != nil {
		return nil, fmt.Errorf("error fetching market summary: %v", err)
	}

	// Extract supplyAmount from market summary
	supplyAmountStr, ok := marketSummary["supplied"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'supplied' field in market summary")
	}

	// Parse supplyAmount as an integer
	supplyAmount, err := strconv.ParseInt(supplyAmountStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing 'supplied' field: %v", err)
	}

	tokenInfo, err := assetData.GetTokenInfo(p.venuePositionConfig.Denom)
	if err != nil {
		return nil, fmt.Errorf("error getting token info: %v", err)
	}

	adjustedAmount := float64(supplyAmount) / math.Pow(10, float64(tokenInfo.Decimals))

	usdValue, atomValue, err := getTokenValues(adjustedAmount, *tokenInfo)
	if err != nil {
		return nil, fmt.Errorf("error calculating token values: %v", err)
	}

	poolAssets := []Asset{
		{
			Denom:       p.venuePositionConfig.Denom,
			Amount:      adjustedAmount,
			USDValue:    usdValue,
			DisplayName: tokenInfo.Display,
		},
	}

	// Return holdings
	return &Holdings{
		Balances:  poolAssets,
		TotalUSDC: usdValue,
		TotalAtom: atomValue,
	}, nil
}

func (p UxPosition) ComputeAddressPrincipalHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	// Construct the query URL
	queryURL := fmt.Sprintf("%s/leverage/v1/account_balances?address=%s", p.protocolConfig.PoolInfoUrl, address)

	// Fetch account balances
	resp, err := http.Get(queryURL)
	if err != nil {
		return nil, fmt.Errorf("fetching account balances: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching account balances: status %d", resp.StatusCode)
	}

	// Decode the response
	var accountBalances map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&accountBalances); err != nil {
		return nil, fmt.Errorf("decoding account balances: %v", err)
	}

	// Extract the "supplied" field
	supplied, ok := accountBalances["supplied"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'supplied' field in account balances")
	}

	// Find the supplied amount for the matching denom
	var suppliedAmount int64
	for _, entry := range supplied {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}

		denom, ok := entryMap["denom"].(string)
		if !ok || denom != p.venuePositionConfig.Denom {
			continue
		}

		amountStr, ok := entryMap["amount"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'amount' field for denom %s", denom)
		}

		suppliedAmount, err = strconv.ParseInt(amountStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing supplied amount: %v", err)
		}
		break
	}

	if suppliedAmount == 0 {
		return nil, fmt.Errorf("no matching supplied amount found for denom %s", p.venuePositionConfig.Denom)
	}

	tokenInfo, err := assetData.GetTokenInfo(p.venuePositionConfig.Denom)
	if err != nil {
		return nil, fmt.Errorf("getting token info: %v", err)
	}

	adjustedAmount := float64(suppliedAmount) / math.Pow(10, float64(tokenInfo.Decimals))

	usdValue, atomValue, err := getTokenValues(adjustedAmount, *tokenInfo)
	if err != nil {
		return nil, fmt.Errorf("calculating token values: %v", err)
	}

	holdingAssets := []Asset{
		{
			Denom:       p.venuePositionConfig.Denom,
			Amount:      adjustedAmount,
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

func (p UxPosition) ComputeAddressRewardHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	// Ux does not have separate reward holdings
	return &Holdings{
		Balances:  []Asset{},
		TotalUSDC: 0,
		TotalAtom: 0,
	}, nil
}

func (p UxPosition) getMarketSummary() (map[string]interface{}, error) {
	queryURL := fmt.Sprintf("%s/leverage/v1/market_summary?denom=%s", p.protocolConfig.PoolInfoUrl, p.venuePositionConfig.Denom)

	resp, err := http.Get(queryURL)
	if err != nil {
		return nil, fmt.Errorf("fetching market summary: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching market summary: status %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decoding response: %v", err)
	}

	return response, nil
}
