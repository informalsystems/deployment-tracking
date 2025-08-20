package main

import (
	"fmt"
)

// Protocol type enum
type Protocol string

const (
	Osmosis          Protocol = "Osmosis"
	Nolus            Protocol = "Nolus"
	Mars             Protocol = "Mars"
	AstroportNeutron Protocol = "Astroport (Neutron)"
	AstroportTerra   Protocol = "Astroport (Terra)"
	Margined         Protocol = "Margined"
	Demex            Protocol = "Demex"
	Neptune          Protocol = "Neptune"
	Shade            Protocol = "Shade"
	WhiteWhale       Protocol = "Whitewhale"
	Inter            Protocol = "Inter"
	Duality          Protocol = "Duality"
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
	InitialAtomAllocation int
	Venues                []VenuePositionConfig
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
	InfoMissing      bool      `json:"info_missing"`
	Protocol         Protocol  `json:"protocol"`
	VenueTotal       *Holdings `json:"venue_total"`
	AddressPrincipal *Holdings `json:"address_holdings"`
	AddressRewards   *Holdings `json:"address_rewards"`
}

type BidHoldings struct {
	BidId                 int             `json:"bid_id"`
	InitialAtomAllocation int             `json:"initial_atom_allocation"`
	Holdings              []VenueHoldings `json:"holdings"`
}

// ExperimentalDeploymentQueryInterface defines the methods required for experimental deployments
type ExperimentalDeploymentQueryInterface interface {
	GetCurrentAddressHoldings(assetData *ChainInfo) (*Holdings, error)
}

type ExperimentalDeployment struct {
	ExperimentalId         int       `json:"experimental_id"`
	Name                   string    `json:"name"`
	Description            string    `json:"description"`
	Logo                   string    `json:"logo"`
	StartTimestamp         int64     `json:"start_timestamp"`
	EndTimestamp           int64     `json:"end_timestamp"`
	InitialAddressHoldings *Holdings `json:"initial_address_holdings"`
	CurrentAddressHoldings *Holdings `json:"current_address_holdings"`
	Querier                ExperimentalDeploymentQueryInterface
}

// ExperimentalDeploymentResponse represents the response structure for experimental deployments
type ExperimentalDeploymentResponse struct {
	ExperimentalId         int       `json:"experimental_id"`
	Name                   string    `json:"name"`
	Description            string    `json:"description"`
	Logo                   string    `json:"logo"`
	StartTimestamp         int64     `json:"start_timestamp"`
	EndTimestamp           int64     `json:"end_timestamp"`
	InitialAddressHoldings *Holdings `json:"initial_address_holdings"`
	CurrentAddressHoldings *Holdings `json:"current_address_holdings"`
}

// experimentalMap holds the configurations for experimental deployments
var experimentalMap = map[int]*ExperimentalDeployment{
	1: {
		ExperimentalId: 1,
		Name:           "Magma: ATOM<>stATOM vault managed by RoboMcGobo",
		Description:    "This is a first experimental deployment to test the Magma vaults integration. The Hydro committee has allocated 10,000 ATOM to this test deployment, which are managed by committee member RoboMcGobo in a [0 fee vault](https://app.magma.eco/vault/osmo1ssm5lqgrxcp9lqvr33zcafyd6unme0q4kq2fpqzgwznnjwujts6sfmfass).",
		Logo:           "https://pbs.twimg.com/profile_images/1830561644285714433/ImSkbXR0_400x400.jpg",
		StartTimestamp: 1742325420,
		EndTimestamp:   0,
		Querier: NewMagmaQuerier(MagmaDeploymentConfig{
			VaultAddress:  "osmo1ssm5lqgrxcp9lqvr33zcafyd6unme0q4kq2fpqzgwznnjwujts6sfmfass",
			HolderAddress: "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
			token0Denom:   "ibc/C140AFD542AE77BD7DCC83F13FDD8C5E5BB8C4929785E6EC2F4C636F98F17901",
			token1Denom:   "ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2",
		}),
		InitialAddressHoldings: &Holdings{
			Balances: []Asset{
				{
					Denom:       "ibc/C140AFD542AE77BD7DCC83F13FDD8C5E5BB8C4929785E6EC2F4C636F98F17901",
					Amount:      1968.1,
					DisplayName: "stATOM",
				},
				{
					Denom:       "ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2",
					Amount:      6976.354,
					DisplayName: "ATOM",
				},
			},
			TotalUSDC: 0, // Will be computed at runtime
			TotalAtom: 0, // Will be computed at runtime
		},
	},
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
	case Margined, Demex, Neptune, Shade, WhiteWhale, Inter:
		return NewMissingPosition(config, venuePositionConfig)
	case Duality:
		return NewDualityPosition(config, venuePositionConfig)
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
	Duality: {
		Protocol:          Duality,
		PoolInfoUrl:       "https://neutron-api.polkachu.com/cosmwasm/wasm/v1/contract",
		AssetListURL:      "https://chains.cosmos.directory/neutron",
		AddressBalanceUrl: "https://neutron-api.polkachu.com/cosmos/bank/v1beta1/balances",
	},
	Margined: {
		Protocol:          Margined,
		PoolInfoUrl:       "",
		AssetListURL:      "",
		AddressBalanceUrl: "",
	},
	Demex: {
		Protocol:          Demex,
		PoolInfoUrl:       "",
		AssetListURL:      "",
		AddressBalanceUrl: "",
	},
	Neptune: {
		Protocol:          Neptune,
		PoolInfoUrl:       "",
		AssetListURL:      "",
		AddressBalanceUrl: "",
	},
	Shade: {
		Protocol:          Shade,
		PoolInfoUrl:       "",
		AssetListURL:      "",
		AddressBalanceUrl: "",
	},
	WhiteWhale: {
		Protocol:          WhiteWhale,
		PoolInfoUrl:       "",
		AssetListURL:      "",
		AddressBalanceUrl: "",
	},
	Inter: {
		Protocol:          Inter,
		PoolInfoUrl:       "",
		AssetListURL:      "",
		AddressBalanceUrl: "",
	},
}

// map of bid ID to its position config
var bidMap = map[int]BidPositionConfig{
	0: {
		InitialAtomAllocation: 10557,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: Margined},
		},
	},
	1: {
		InitialAtomAllocation: 10000,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: Demex},
		},
	},
	2: {
		InitialAtomAllocation: 18000,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: Neptune},
		},
	},
	3: {
		InitialAtomAllocation: 50000,
		Venues: []VenuePositionConfig{
			// TODO: can't differentiate from bids 11 and 24 deployments
			AstroportVenuePositionConfig{
				Protocol:         AstroportNeutron,
				PoolAddress:      "neutron1yem82r0wf837lfkwvcu2zxlyds5qrzwkz8alvmg0apyrjthk64gqeq2e98",
				IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqgknctjyq6m3tch",
				Address:          "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
			},
		},
	},
	4: {
		InitialAtomAllocation: 36093,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: Shade},
		},
	},
	5: {
		InitialAtomAllocation: 10000,
		Venues: []VenuePositionConfig{
			NolusVenuePositionConfig{
				PoolContractAddress: "nolus1jufcaqm6657xmfltdezzz85quz92rmtd88jk5x0hq9zqseem32ysjdm990",
				PoolContractToken:   NOLUS_ST_ATOM,
				Address:             "nolus1u74s6nuqgulf9kuezjt9q8r8ghx0kcvcl96fx63nk29df25n2u5swmz3g6",
			},
		},
	},
	6: {
		InitialAtomAllocation: 3143,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: WhiteWhale},
		},
	},
	7: {
		InitialAtomAllocation: 17912,
		Venues: []VenuePositionConfig{
			// TODO: can't differentiate from bid 15 deployment
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
			},
		},
	},
	// round 2 starts here
	11: {
		InitialAtomAllocation: 81000,
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
			},
		},
	},
	12: {
		InitialAtomAllocation: 33953,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: Shade},
		},
	},
	14: {
		InitialAtomAllocation: 10000,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: WhiteWhale},
		},
	},
	15: {
		InitialAtomAllocation: 26000,
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
			},
		},
	},
	16: {
		InitialAtomAllocation: 42000,
		Venues: []VenuePositionConfig{
			MarsVenuePositionConfig{
				CreditAccountID: "2533",
				DepositedDenom:  NEUTRON_ATOM,
			},
		},
	},
	17: {
		InitialAtomAllocation: 48650,
		Venues: []VenuePositionConfig{
			OsmosisVenuePositionConfig{
				PoolID:     "1283",
				Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
				PositionID: "11124334",
			},
		},
	},
	// round 3 starts here
	18: {
		InitialAtomAllocation: 45780,
		Venues: []VenuePositionConfig{
			OsmosisVenuePositionConfig{
				PoolID:     "1283",
				Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
				PositionID: "11701290",
			},
		},
	},
	19: {
		InitialAtomAllocation: 69171,
		Venues: []VenuePositionConfig{
			// TODO: populate once it gets deployed
			// OsmosisVenuePositionConfig{
			// 	PoolID:     "1283",
			// 	Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
			// 	PositionID: "",
			// },
			MissingVenuePositionConfig{Protocol: Inter},
		},
	},
	22: {
		InitialAtomAllocation: 10000,
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
			},
		},
	},
	23: {
		InitialAtomAllocation: 22340,
		Venues: []VenuePositionConfig{
			NolusVenuePositionConfig{
				PoolContractAddress: "nolus1u0zt8x3mkver0447glfupz9lz6wnt62j70p5fhhtu3fr46gcdd9s5dz9l6",
				PoolContractToken:   NOLUS_ATOM,
				Address:             "nolus1u74s6nuqgulf9kuezjt9q8r8ghx0kcvcl96fx63nk29df25n2u5swmz3g6",
			},
		},
	},
	24: {
		InitialAtomAllocation: 43962,
		Venues: []VenuePositionConfig{
			MarsVenuePositionConfig{
				CreditAccountID: "3091",
				DepositedDenom:  NEUTRON_ATOM,
			},
			AstroportVenuePositionConfig{
				Protocol:         AstroportNeutron,
				PoolAddress:      "neutron1yem82r0wf837lfkwvcu2zxlyds5qrzwkz8alvmg0apyrjthk64gqeq2e98",
				IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqg4gknctjyq6m3tch",
				Address:          "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
			},
		},
	},
	25: {
		InitialAtomAllocation: 170000,
		Venues: []VenuePositionConfig{
			OsmosisVenuePositionConfig{
				PoolID:     "1283",
				Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
				PositionID: "12515115",
			},
		},
	},
	26: {
		InitialAtomAllocation: 0,
		Venues: []VenuePositionConfig{
			NolusVenuePositionConfig{
				PoolContractAddress: "nolus1u0zt8x3mkver0447glfupz9lz6wnt62j70p5fhhtu3fr46gcdd9s5dz9l6",
				PoolContractToken:   NOLUS_ATOM,
				Address:             "nolus1u74s6nuqgulf9kuezjt9q8r8ghx0kcvcl96fx63nk29df25n2u5swmz3g6",
			},
		},
	},
	51: {
		InitialAtomAllocation: 78000,
		Venues: []VenuePositionConfig{
			DualityVenuePositionConfig{
				PoolAddress: "neutron18ua532r8lpy8scvysrgcjneyrwuj4x0ne4t2azphxksya596l4cq23lkp9",
				Address:     "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
			},
		},
	},
}
