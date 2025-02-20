package main

import "fmt"

// Protocol type enum
type Protocol string

const (
	Osmosis   Protocol = "osmosis"
	Astroport Protocol = "astroport"
	Nolus     Protocol = "nolus"
)

// Core data structures

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
	ComputeTVL(assetData map[string]interface{}) (*Holdings, error)
	ComputeAddressPrincipalHoldings(assetData map[string]interface{}, address string) (*Holdings, error)
	ComputeAddressRewardHoldings(assetData map[string]interface{}, address string) (*Holdings, error)
}

func NewDexProtocolFromConfig(config ProtocolConfig, bidPositionConfig BidPositionConfig) (DexProtocol, error) {
	switch config.Protocol {
	case Osmosis:
		return NewOsmosisPosition(config, bidPositionConfig)
	case Nolus:
		return NewNolusPosition(config, bidPositionConfig)
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
}
