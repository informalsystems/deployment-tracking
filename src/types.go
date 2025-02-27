package main

import "fmt"

// Protocol type enum
type Protocol string

const (
	Osmosis          Protocol = "osmosis"
	Astroport        Protocol = "astroport"
	Nolus            Protocol = "nolus"
	Mars             Protocol = "mars"
	AstroportNeutron Protocol = "astroport-neutron"
	AstroportTerra   Protocol = "astroport-terra"
)

// Core data structures

type ChainTokenInfo struct {
	Denom       string `json:"denom"`
	Display     string `json:"display"`
	Decimals    int    `json:"decimals"`
	CoingeckoID string `json:"coingecko_id"`
}

type ChainInfo struct {
	ChainID string                    `json:"chain_id"`
	Tokens  map[string]ChainTokenInfo `json:"tokens"` // denom -> info
}

func (c *ChainInfo) GetTokenInfo(denom string) (*ChainTokenInfo, error) {
	if info, ok := c.Tokens[denom]; ok {
		return &info, nil
	}
	return nil, fmt.Errorf("token info not found for denom: %s", denom)
}

// PositionConfig holds the configuration for
// a single bid position.
// It contains the protocol the position is on,
// as well as all the information to identify a
// concrete position, namely the pool ID,
// address the position is associated with, and position ID.
type BidPositionConfig interface {
	GetPoolID() string
	GetAddress() string
	GetProtocol() Protocol
}

// ProtocolConfig holds the configuration for a protocol, independent
// of the position we query in that protocol.
type ProtocolConfig struct {
	AssetListURL      string
	PoolInfoUrl       string
	AddressBalanceUrl string
	Protocol          Protocol
}

type Asset struct {
	Denom       string  `json:"denom"`
	Amount      float64 `json:"amount"`
	CoingeckoID *string `json:"coingecko_id,omitempty"`
	USDValue    float64 `json:"usd_value"`
	DisplayName string  `json:"display_name,omitempty"`
}

type Holdings struct {
	Balances  []Asset `json:"balances"`
	TotalUSDC float64 `json:"total_usdc"`
	TotalAtom float64 `json:"total_atom"`
}

type VenueHoldings struct {
	VenueTotal       Holdings `json:"venue_total"`
	AddressPrincipal Holdings `json:"address_holdings"`
	AddressRewards   Holdings `json:"address_rewards"`
}

// Protocol interface
type DexProtocol interface {
	ComputeTVL(assetData *ChainInfo) (*Holdings, error)
	ComputeAddressPrincipalHoldings(assetData *ChainInfo, address string) (*Holdings, error)
	ComputeAddressRewardHoldings(assetData *ChainInfo, address string) (*Holdings, error)
}

func NewDexProtocolFromConfig(config ProtocolConfig, bidPositionConfig BidPositionConfig) (DexProtocol, error) {
	switch config.Protocol {
	case Osmosis:
		return NewOsmosisPosition(config, bidPositionConfig)
	case Nolus:
		return NewNolusPosition(config, bidPositionConfig)
	case Mars:
		return NewMarsPosition(config, bidPositionConfig)
	case AstroportNeutron, AstroportTerra:
		return NewAstroportPosition(config, bidPositionConfig)
	}

	return nil, fmt.Errorf("unsupported protocol: %s", config.Protocol)
}

var protocolConfigMap = map[Protocol]ProtocolConfig{
	Osmosis: {
		Protocol:          Osmosis,
		PoolInfoUrl:       "https://sqs.osmosis.zone",
		AssetListURL:      "https://chains.cosmos.directory/osmosis",
		AddressBalanceUrl: "https://lcd.osmosis.zone/",
	},
	Nolus: {
		Protocol:          Nolus,
		PoolInfoUrl:       "https://nolus-api.polkachu.com/cosmwasm/wasm/v1/contract",
		AssetListURL:      "https://chains.cosmos.directory/nolus",
		AddressBalanceUrl: "",
	},
	Mars: {
		Protocol:          Mars,
		PoolInfoUrl:       "https://neutron-api.polkachu.com/cosmwasm/wasm/v1/contract",
		AssetListURL:      "https://chains.cosmos.directory/neutron",
		AddressBalanceUrl: "",
	},
	AstroportNeutron: {
		Protocol:          AstroportNeutron,
		PoolInfoUrl:       "https://neutron-api.polkachu.com/cosmwasm/wasm/v1/contract",
		AssetListURL:      "https://chains.cosmos.directory/neutron",
		AddressBalanceUrl: "https://neutron-api.polkachu.com/cosmos/bank/v1beta1/balances",
	},
	AstroportTerra: {
		Protocol:          AstroportTerra,
		PoolInfoUrl:       "https://terra-api.polkachu.com/cosmwasm/wasm/v1/contract",
		AssetListURL:      "https://chains.cosmos.directory/terra",
		AddressBalanceUrl: "https://terra-api.polkachu.com/cosmos/bank/v1beta1/balances",
	},
}

// map of bid id to protocol and pool id, position id, address
var bidMap = map[string]BidPositionConfig{
	"18": OsmosisBidPositionConfig{
		PoolID:     "1283",
		Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
		PositionID: "11701290",
	},
	"17": OsmosisBidPositionConfig{
		PoolID:     "1283",
		Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
		PositionID: "11124334",
	},
	"11.osmosis": OsmosisBidPositionConfig{
		PoolID:     "2371",
		Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
		PositionID: "11124249",
	},
	"11.astroport": AstroportBidPositionConfig{
		ChainID:          "neutron-1",
		PoolAddress:      "neutron1yem82r0wf837lfkwvcu2zxlyds5qrzwkz8alvmg0apyrjthk64gqeq2e98",
		IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqg4gknctjyq6m3tch",
		Address:          "neutron14fmxw54lgvheyn7m0p9efpr82fac68ysph96ch",
	},
	"5": NolusBidPositionConfig{
		PoolContractAddress: "nolus1jufcaqm6657xmfltdezzz85quz92rmtd88jk5x0hq9zqseem32ysjdm990",
		PoolContractToken:   NOLUS_ST_ATOM,
		Address:             "nolus1u74s6nuqgulf9kuezjt9q8r8ghx0kcvcl96fx63nk29df25n2u5swmz3g6",
	},
	"23": NolusBidPositionConfig{
		PoolContractAddress: "nolus1u0zt8x3mkver0447glfupz9lz6wnt62j70p5fhhtu3fr46gcdd9s5dz9l6",
		PoolContractToken:   NOLUS_ATOM,
		Address:             "nolus1u74s6nuqgulf9kuezjt9q8r8ghx0kcvcl96fx63nk29df25n2u5swmz3g6",
	},
	"16": MarsBidPositionConfig{
		CreditAccountID: "2533",
		DepositedDenom:  NEUTRON_ATOM,
	},
	"24.mars": MarsBidPositionConfig{
		CreditAccountID: "3091",
		DepositedDenom:  NEUTRON_ATOM,
	},
}
