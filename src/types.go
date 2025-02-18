package main

import "fmt"

// Protocol type enum
type Protocol string

const (
	Osmosis   Protocol = "osmosis"
	Astroport Protocol = "astroport"
)

// Core data structures

// PositionConfig holds the configuration for
// a single bid position.
// It contains the protocol the position is on,
// as well as all the information to identify a
// concrete position, namely the pool ID,
// address the position is associated with, and position ID.
type BidPositionConfig struct {
	PoolID     string
	Address    string
	PositionID string
	Protocol   Protocol
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
	DisplayName *string `json:"display_name,omitempty"`
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
		return NewOsmosisPosition(config, bidPositionConfig), nil
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
}

// map of bid id to protocol and pool id, position id, address
var bidMap = map[string]BidPositionConfig{
	"18": {
		Protocol:   Osmosis,
		PoolID:     "1283",
		Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
		PositionID: "11701290",
	},
	"17": {
		Protocol:   Osmosis,
		PoolID:     "1283",
		Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
		PositionID: "11124334",
	},
	"11.osmosis": {
		Protocol:   Osmosis,
		PoolID:     "2371",
		Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
		PositionID: "11124249",
	},
}
