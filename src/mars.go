package main

import (
	"fmt"
	"math"
	"strconv"
)

const (
	NEUTRON_ATOM                    = "ibc/C4CFF46FD6DE35CA4CF4CE031E643C8FDC9BA4B99AE598E9B0ED98FE3A2319F9"
	CREDIT_MANAGER_CONTRACT_ADDRESS = "neutron1qdzn3l4kn7gsjna2tfpg3g3mwd6kunx4p50lfya59k02846xas6qslgs3r"
	PARAMS_CONTRACT_ADDRESS         = "neutron1x4rgd7ry23v2n49y7xdzje0743c5tgrnqrqsvwyya2h6m48tz4jqqex06x"
)

type MarsBidPositionConfig struct {
	CreditAccountID string
	DepositedDenom  string
}

func (bidConfig MarsBidPositionConfig) GetProtocol() Protocol {
	return Mars
}

func (bidConfig MarsBidPositionConfig) GetPoolID() string {
	return CREDIT_MANAGER_CONTRACT_ADDRESS
}

func (bidConfig MarsBidPositionConfig) GetAddress() string {
	return bidConfig.CreditAccountID
}

type MarsPosition struct {
	protocolConfig    ProtocolConfig
	bidPositionConfig MarsBidPositionConfig
}

func NewMarsPosition(config ProtocolConfig, bidPositionConfig BidPositionConfig) (*MarsPosition, error) {
	marsBidPositionConfig, ok := bidPositionConfig.(MarsBidPositionConfig)
	if !ok {
		return nil, fmt.Errorf("bidPositionConfig must be of MarsBidPositionConfig type")
	}

	return &MarsPosition{protocolConfig: config, bidPositionConfig: marsBidPositionConfig}, nil
}

func (p MarsPosition) ComputeTVL(assetData map[string]interface{}) (*Holdings, error) {
	return p.computeHoldings(assetData, p.getTotalDepositInPool)
}

func (p MarsPosition) ComputeAddressPrincipalHoldings(assetData map[string]interface{}, address string) (*Holdings, error) {
	return p.computeHoldings(assetData, p.getCreditAccountDepositInPool)
}

func (p MarsPosition) ComputeAddressRewardHoldings(assetData map[string]interface{}, address string) (*Holdings, error) {
	// rewards are already counted-in into principal address holdings, since Mars protocol doesn't keep track of
	// the initial holdings and yield separately
	return p.computeHoldings(assetData, func() (int, error) { return 0, nil })
}

func (p MarsPosition) computeHoldings(assetData map[string]interface{}, getTokenAmountFunc func() (int, error)) (*Holdings, error) {
	mapping, err := buildTokenMapping(assetData)
	if err != nil {
		return nil, fmt.Errorf("building token mapping: %s", err)
	}

	poolToken := p.bidPositionConfig.DepositedDenom
	tokenInfo, err := mapping.GetTokenInfo(poolToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get token info for %s. Error: %s", poolToken, err)
	}

	tokenAmount, err := getTokenAmountFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to load token amount: %s", err)
	}

	adjustedTokenAmount := float64(tokenAmount) / math.Pow(10, float64(tokenInfo.Decimals))
	totalValueUSD, totalValueAtom, err := getTokenValues(adjustedTokenAmount, *tokenInfo, assetData)
	if err != nil {
		return nil, fmt.Errorf("failed to compute token values: %s", err)
	}

	holdings := Holdings{
		Balances: []Asset{
			{
				Denom:       poolToken,
				Amount:      adjustedTokenAmount,
				CoingeckoID: nil,
				USDValue:    totalValueUSD,
				DisplayName: tokenInfo.DisplayName,
			},
		},
		TotalUSDC: totalValueUSD,
		TotalAtom: totalValueAtom,
	}

	return &holdings, nil
}

func (p MarsPosition) getTotalDepositInPool() (int, error) {
	queryJson := map[string]interface{}{
		"total_deposit": struct {
			Denom string `json:"denom"`
		}{Denom: p.bidPositionConfig.DepositedDenom},
	}

	data, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl, PARAMS_CONTRACT_ADDRESS, queryJson)
	if err != nil {
		return 0, err
	}

	amountStr, ok := data["amount"].(string)
	if !ok {
		return 0, fmt.Errorf("invalid pool total deposit")
	}

	return strconv.Atoi(amountStr)
}

func (p MarsPosition) getCreditAccountDepositInPool() (int, error) {
	queryJson := map[string]interface{}{
		"positions": struct {
			AccountID string `json:"account_id"`
		}{AccountID: p.bidPositionConfig.CreditAccountID},
	}

	data, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl, CREDIT_MANAGER_CONTRACT_ADDRESS, queryJson)
	if err != nil {
		return 0, err
	}

	lends, ok := data["lends"].([]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid credit account lend positions")
	}

	for _, lend := range lends {
		lendStruct, ok := lend.(map[string]interface{})
		if !ok {
			return 0, fmt.Errorf("invalid credit account lend position")
		}

		lendDenom := lendStruct["denom"].(string)
		if lendDenom != p.bidPositionConfig.DepositedDenom {
			continue
		}

		lendAmountStr := lendStruct["amount"].(string)

		return strconv.Atoi(lendAmountStr)
	}

	return 0, fmt.Errorf("no position found for credit account ID: %s and denom: %s", p.bidPositionConfig.CreditAccountID, p.bidPositionConfig.DepositedDenom)
}
