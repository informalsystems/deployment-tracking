package main

import "fmt"

// Protocol type enum
type Protocol string

const (
	Osmosis          Protocol = "osmosis"
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

// BidPositionConfig holds configuration for all venue positions of the given bid.
type BidPositionConfig struct {
	InitialAtomDeposit int
	Venues             []VenuePositionConfig
}

// VenuePositionConfig holds the configuration for
// a single venue position.
// It contains the protocol the position is on,
// as well as all the information to identify a
// concrete position, namely the pool ID and
// address the position is associated with.
type VenuePositionConfig interface {
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

type BidHoldings struct {
	BidId    int             `json:"bid_id"`
	Holdings []VenueHoldings `json:"holdings"`
}

// Protocol interface
type DexProtocol interface {
	ComputeTVL(assetData *ChainInfo) (*Holdings, error)
	ComputeAddressPrincipalHoldings(assetData *ChainInfo, address string) (*Holdings, error)
	ComputeAddressRewardHoldings(assetData *ChainInfo, address string) (*Holdings, error)
}

func NewDexProtocolFromConfig(config ProtocolConfig, venuePositionConfig VenuePositionConfig) (DexProtocol, error) {
	switch config.Protocol {
	case Osmosis:
		return NewOsmosisPosition(config, venuePositionConfig)
	case Nolus:
		return NewNolusPosition(config, venuePositionConfig)
	case Mars:
		return NewMarsPosition(config, venuePositionConfig)
	case AstroportNeutron, AstroportTerra:
		return NewAstroportPosition(config, venuePositionConfig)
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
		AssetListURL:      "https://chains.cosmos.directory/terra2",
		AddressBalanceUrl: "https://terra-api.polkachu.com/cosmos/bank/v1beta1/balances",
	},
}

// map of bid ID to its position config
var bidMap = map[int]BidPositionConfig{
	17: {
		InitialAtomDeposit: 48650,
		Venues: []VenuePositionConfig{OsmosisVenuePositionConfig{
			PoolID:     "1283",
			Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
			PositionID: "11124334",
		}},
	},
	18: {
		InitialAtomDeposit: 45000,
		Venues: []VenuePositionConfig{OsmosisVenuePositionConfig{
			PoolID:     "1283",
			Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
			PositionID: "11701290",
		}},
	},
	11: {
		InitialAtomDeposit: 81000,
		Venues: []VenuePositionConfig{
			OsmosisVenuePositionConfig{
				PoolID:     "2371",
				Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
				PositionID: "11124249",
			},
			AstroportVenuePositionConfig{
				Protocol:         AstroportNeutron,
				PoolAddress:      "neutron1yem82r0wf837lfkwvcu2zxlyds5qrzwkz8alvmg0apyrjthk64gqeq2e98",
				IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqg4gknctjyq6m3tch",
				Address:          "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
			}},
	},
	5: {
		InitialAtomDeposit: 10000,
		Venues: []VenuePositionConfig{NolusVenuePositionConfig{
			PoolContractAddress: "nolus1jufcaqm6657xmfltdezzz85quz92rmtd88jk5x0hq9zqseem32ysjdm990",
			PoolContractToken:   NOLUS_ST_ATOM,
			Address:             "nolus1u74s6nuqgulf9kuezjt9q8r8ghx0kcvcl96fx63nk29df25n2u5swmz3g6",
		}},
	},
	23: {
		InitialAtomDeposit: 22340,
		Venues: []VenuePositionConfig{NolusVenuePositionConfig{
			PoolContractAddress: "nolus1u0zt8x3mkver0447glfupz9lz6wnt62j70p5fhhtu3fr46gcdd9s5dz9l6",
			PoolContractToken:   NOLUS_ATOM,
			Address:             "nolus1u74s6nuqgulf9kuezjt9q8r8ghx0kcvcl96fx63nk29df25n2u5swmz3g6",
		}},
	},
	16: {
		InitialAtomDeposit: 42000,
		Venues: []VenuePositionConfig{MarsVenuePositionConfig{
			CreditAccountID: "2533",
			DepositedDenom:  NEUTRON_ATOM,
		}},
	},
	24: {
		InitialAtomDeposit: 21981, // TODO: add other venues
		Venues: []VenuePositionConfig{MarsVenuePositionConfig{
			// 21981 to Mars Protocol
			CreditAccountID: "3091",
			DepositedDenom:  NEUTRON_ATOM,
		}},
	},
	7: {
		InitialAtomDeposit: 17912,
		Venues: []VenuePositionConfig{
			AstroportVenuePositionConfig{
				Protocol:         AstroportTerra,
				PoolAddress:      "terra1f9vmtntpjmkyhkxtlc49jcq6cv8rfz0kr06zv6efdtdgae4m9y9qlzm36t",
				IncentiveAddress: "terra1eywh4av8sln6r45pxq45ltj798htfy0cfcf7fy3pxc2gcv6uc07se4ch9x",
				Address:          "terra12wq57ea7m7m8wx4qhsj04fyc78pv2n3h888vfzuv7n7k7qlq2dyssuyf8h",
			},
			AstroportVenuePositionConfig{
				Protocol:         AstroportTerra,
				PoolAddress:      "terra1a0h6vrzkztjystg8sd949qyrc6mw9gzxk2870cr2mukg53uzgvqs46qul9",
				IncentiveAddress: "terra1eywh4av8sln6r45pxq45ltj798htfy0cfcf7fy3pxc2gcv6uc07se4ch9x",
				Address:          "terra12wq57ea7m7m8wx4qhsj04fyc78pv2n3h888vfzuv7n7k7qlq2dyssuyf8h",
			}},
	},
	22: {
		InitialAtomDeposit: 10000,
		Venues: []VenuePositionConfig{
			AstroportVenuePositionConfig{
				Protocol:         AstroportNeutron,
				PoolAddress:      "neutron14y0xyavpf5xznw56u3xml9f2jmx8ruk3y8f5e6zzkd9mhmcps3fs59g4vt",
				IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqg4gknctjyq6m3tch",
				Address:          "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
			},
			AstroportVenuePositionConfig{
				Protocol:         AstroportNeutron,
				PoolAddress:      "neutron1w8vmg3zwyh62edp7uxpaw90447da9zzlv0kqh2ajye6a6mseg06qseyv5m",
				IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqg4gknctjyq6m3tch",
				Address:          "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
			}},
	},
}
