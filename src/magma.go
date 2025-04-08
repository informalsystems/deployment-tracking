package main

import (
	"fmt"
	"math"
	"strconv"
)

// MagmaDeploymentConfig holds the configuration for a Magma deployment
type MagmaDeploymentConfig struct {
	// The address whose holdings in the Magma vault we want to query.
	HolderAddress string
	// The address of the Magma vault.
	VaultAddress string
	// The denom of the first asset in the vault.
	token0Denom string
	// The denom of the second asset in the vault.
	token1Denom string
}

// MagmaHoldingsData represents the response from Magma's API
type MagmaHoldingsData struct {
	Amount     string `json:"amount"`
	ShareRatio string `json:"share_ratio"`
}

// MagmaDeployment implements ExperimentalDeploymentQueryInterface
type MagmaQuerier struct {
	config MagmaDeploymentConfig
}

func NewMagmaQuerier(config MagmaDeploymentConfig) *MagmaQuerier {
	return &MagmaQuerier{
		config: config,
	}
}

func (m *MagmaQuerier) computeHoldings(assetData *ChainInfo) (*Holdings, error) {
	nodeURL := "https://osmosis-lcd.numia.xyz/cosmwasm/wasm/v1/contract/"

	// 1. Query balance of vault shares
	balanceQuery := map[string]interface{}{
		"balance": map[string]interface{}{
			"address": m.config.HolderAddress,
		},
	}

	balanceData, err := QuerySmartContractData(nodeURL, m.config.VaultAddress, balanceQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query balance: %v", err)
	}

	balance, ok := balanceData.(map[string]interface{})["balance"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid balance response format")
	}

	holderBalance, err := strconv.ParseFloat(balance, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse holder balance: %v", err)
	}

	// 2. Query token info for total supply
	tokenInfoQuery := map[string]interface{}{
		"token_info": map[string]interface{}{},
	}

	tokenInfoData, err := QuerySmartContractData(nodeURL, m.config.VaultAddress, tokenInfoQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query token info: %v", err)
	}

	tokenInfo, ok := tokenInfoData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid token info response format")
	}

	totalSupply, err := strconv.ParseFloat(tokenInfo["total_supply"].(string), 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse total supply: %v", err)
	}

	// Calculate share ratio
	shareRatio := holderBalance / totalSupply

	// 3. Query vault balances
	vaultBalancesQuery := map[string]interface{}{
		"vault_balances": map[string]interface{}{},
	}

	vaultBalancesData, err := QuerySmartContractData(nodeURL, m.config.VaultAddress, vaultBalancesQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query vault balances: %v", err)
	}

	vaultBalances, ok := vaultBalancesData.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid vault balances response format")
	}

	bal0, err := strconv.ParseFloat(vaultBalances["bal0"].(string), 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bal0: %v", err)
	}

	bal1, err := strconv.ParseFloat(vaultBalances["bal1"].(string), 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bal1: %v", err)
	}

	// Calculate user's share of each asset
	userBal0 := bal0 * shareRatio
	userBal1 := bal1 * shareRatio

	token0Denom := m.config.token0Denom
	token1Denom := m.config.token1Denom

	// Get token info for both assets
	token0Info, err := assetData.GetTokenInfo(token0Denom)
	if err != nil {
		return nil, fmt.Errorf("token info not found for %s: %v", token0Denom, err)
	}

	token1Info, err := assetData.GetTokenInfo(token1Denom)
	if err != nil {
		return nil, fmt.Errorf("token info not found for %s: %v", token1Denom, err)
	}

	// Adjust amounts for decimals
	adjustedBal0 := userBal0 / math.Pow10(int(token0Info.Decimals))
	adjustedBal1 := userBal1 / math.Pow10(int(token1Info.Decimals))

	// Get USD prices using Numia API
	price0, err := getNumiaPrice(token0Denom)
	if err != nil {
		return nil, fmt.Errorf("failed to get token0 price: %v", err)
	}

	price1, err := getNumiaPrice(token1Denom)
	if err != nil {
		return nil, fmt.Errorf("failed to get token1 price: %v", err)
	}

	// Calculate USD values
	usdValue0 := adjustedBal0 * price0
	usdValue1 := adjustedBal1 * price1

	// Get ATOM price for conversion
	atomPrice, err := getNumiaPrice("ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2")
	if err != nil {
		return nil, fmt.Errorf("failed to get ATOM price: %v", err)
	}

	// Calculate ATOM values
	atomValue0 := usdValue0 / atomPrice
	atomValue1 := usdValue1 / atomPrice

	holdings := &Holdings{
		Balances: []Asset{
			{
				Denom:       token0Denom,
				Amount:      adjustedBal0,
				CoingeckoID: nil,
				USDValue:    usdValue0,
				DisplayName: token0Info.Display,
			},
			{
				Denom:       token1Denom,
				Amount:      adjustedBal1,
				CoingeckoID: nil,
				USDValue:    usdValue1,
				DisplayName: token1Info.Display,
			},
		},
		TotalUSDC: usdValue0 + usdValue1,
		TotalAtom: atomValue0 + atomValue1,
	}

	return holdings, nil
}

func (m *MagmaQuerier) GetCurrentAddressHoldings(assetData *ChainInfo) (*Holdings, error) {
	holdings, err := m.computeHoldings(assetData)
	if err != nil {
		debugLog("Error computing Magma holdings", map[string]string{"error": err.Error()})
	}
	return holdings, err
}
