package main

import (
	"fmt"
	"math"
	"strconv"
)

const (
	NOLUS_ATOM    = "ibc/6CDD4663F2F09CD62285E2D45891FC149A3568E316CE3EBBE201A71A78A69388"
	NOLUS_ST_ATOM = "ibc/FCFF8B19C61677F3B78E2A5AE3B4A34A8D23858D16905F253B8438B3AFD07FF8"
)

type NolusBidPositionConfig struct {
	PoolContractAddress string
	PoolContractToken   string
	Address             string
}

func (bidConfig NolusBidPositionConfig) GetProtocol() Protocol {
	return Nolus
}

func (bidConfig NolusBidPositionConfig) GetPoolID() string {
	return bidConfig.PoolContractAddress
}

func (bidConfig NolusBidPositionConfig) GetAddress() string {
	return bidConfig.Address
}

type NolusPosition struct {
	protocolConfig    ProtocolConfig
	bidPositionConfig NolusBidPositionConfig
}

func NewNolusPosition(config ProtocolConfig, bidPositionConfig BidPositionConfig) (*NolusPosition, error) {
	nolusBidPositionConfig, ok := bidPositionConfig.(NolusBidPositionConfig)
	if !ok {
		return nil, fmt.Errorf("bidPositionConfig must be of NolusBidPositionConfig type")
	}

	return &NolusPosition{protocolConfig: config, bidPositionConfig: nolusBidPositionConfig}, nil
}

func (p NolusPosition) ComputeTVL(assetData *ChainInfo) (*Holdings, error) {
	return p.computeHoldings(assetData, p.getTotalPoolShares)
}

func (p NolusPosition) ComputeAddressPrincipalHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	return p.computeHoldings(assetData, func() (int, error) { return p.getAddressBalanceShares(address) })
}

func (p NolusPosition) ComputeAddressRewardHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	return p.computeHoldings(assetData, func() (int, error) { return p.getAddressRewardsShares(address) })
}

func (p NolusPosition) computeHoldings(assetData *ChainInfo, getSharesFunc func() (int, error)) (*Holdings, error) {
	poolToken := p.bidPositionConfig.PoolContractToken

	tokenInfo, ok := assetData.Tokens[poolToken]
	if !ok {
		return nil, fmt.Errorf("token info not found for %s", poolToken)
	}

	tokenShares, err := getSharesFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to load pool shares: %s", err.Error())
	}

	ratio, err := p.getShareToTokenRatio()
	if err != nil {
		return nil, fmt.Errorf("failed to load share to token ratio: %s", err.Error())
	}

	rawTokenAmount := float64(tokenShares) * ratio
	adjustedTokenAmount := rawTokenAmount / math.Pow(10, float64(tokenInfo.Decimals))

	totalValueUSD, totalValueAtom, err := getTokenValues(adjustedTokenAmount, tokenInfo)
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
				DisplayName: tokenInfo.Display,
			},
		},
		TotalUSDC: totalValueUSD,
		TotalAtom: totalValueAtom,
	}

	return &holdings, nil
}

func (p NolusPosition) getShareToTokenRatio() (float64, error) {
	queryJson := map[string]interface{}{
		"price": []interface{}{},
	}

	data, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl, p.bidPositionConfig.PoolContractAddress, queryJson)
	if err != nil {
		return 0, err
	}

	amountStr, ok := data["amount"].(map[string]interface{})["amount"].(string)
	if !ok {
		return 0, fmt.Errorf("invalid pool balance structure")
	}

	amountQuoteStr, ok := data["amount_quote"].(map[string]interface{})["amount"].(string)
	if !ok {
		return 0, fmt.Errorf("invalid pool balance structure")
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse amount into float64: %s", err)
	}

	amountQuote, err := strconv.ParseFloat(amountQuoteStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse amount_quote into float64: %s", err)
	}

	return amountQuote / amount, nil
}

func (p NolusPosition) getTotalPoolShares() (int, error) {
	queryJson := map[string]interface{}{
		"lpp_balance": []interface{}{},
	}

	data, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl, p.bidPositionConfig.PoolContractAddress, queryJson)
	if err != nil {
		return 0, err
	}

	balanceShares, ok := data["balance_nlpn"].(map[string]interface{})["amount"].(string)
	if !ok {
		return 0, fmt.Errorf("invalid balance_nlpn")
	}

	poolBalance, err := strconv.Atoi(balanceShares)
	return poolBalance, err
}

func (p NolusPosition) getAddressBalanceShares(address string) (int, error) {
	queryJson := map[string]interface{}{
		"balance": struct {
			Address string `json:"address"`
		}{Address: address},
	}

	data, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl, p.bidPositionConfig.PoolContractAddress, queryJson)
	if err != nil {
		return 0, err
	}

	balanceShares, ok := data["balance"].(string)
	if !ok {
		return 0, fmt.Errorf("invalid balance")
	}

	addressBalance, err := strconv.Atoi(balanceShares)
	return addressBalance, err
}

func (p NolusPosition) getAddressRewardsShares(address string) (int, error) {
	queryJson := map[string]interface{}{
		"rewards": struct {
			Address string `json:"address"`
		}{Address: address},
	}

	data, err := QuerySmartContractData(p.protocolConfig.PoolInfoUrl, p.bidPositionConfig.PoolContractAddress, queryJson)
	if err != nil {
		return 0, err
	}

	addressRewardsShares, ok := data["rewards"].(map[string]interface{})["amount"].(string)
	if !ok {
		return 0, fmt.Errorf("invalid balance")
	}

	addressRewards, err := strconv.Atoi(addressRewardsShares)
	return addressRewards, err
}
