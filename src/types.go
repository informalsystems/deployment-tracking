package main

import (
	"fmt"
	"time"
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
	Elys             Protocol = "Elys"
	Duality          Protocol = "Duality"
	Ux               Protocol = "Ux"
	Pryzm            Protocol = "Pryzm"
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
	InitialAllocation int                   `json:"initial_allocation"`
	Venues            []VenuePositionConfig `json:"venues"`
	Withdrawals       []Withdrawal          `json:"withdrawals"`
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
	BidId             int             `json:"bid_id"`
	InitialAllocation int             `json:"initial_allocation"`
	Holdings          []VenueHoldings `json:"holdings"`
	Withdrawals       []Withdrawal    `json:"withdrawals"`
}

type Withdrawal struct {
	Date            time.Time `json:"date"`              // Date of the withdrawal
	WithdrawnAmount float64   `json:"withdrawn_amount"`  // Amount of withdrawal
	WithdrawnShares float64   `json:"withdrawn_shares"`  // Amount of shares withdrawn (if applicable)
	CompoundedBidId int       `json:"compounded_bid_id"` // ID of the compounded bid
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
	case Elys:
		return NewElysPosition(config, venuePositionConfig)
	case Neptune:
		return NewNeptunePosition(config, venuePositionConfig)
	case Margined, Demex, Shade, WhiteWhale, Inter, Pryzm:
		return NewMissingPosition(config, venuePositionConfig)
	case Duality:
		return NewDualityPosition(config, venuePositionConfig)
	case Ux:
		return NewUxPosition(config, venuePositionConfig)
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
	Elys: {
		Protocol:          Elys,
		PoolInfoUrl:       "https://elys-rest.publicnode.com/elys-network/elys",
		AssetListURL:      "https://chains.cosmos.directory/elys",
		AddressBalanceUrl: "",
	},
	Duality: {
		Protocol:          Duality,
		PoolInfoUrl:       "https://api.neutron.quokkastake.io/cosmwasm/wasm/v1/contract",
		AssetListURL:      "https://chains.cosmos.directory/neutron",
		AddressBalanceUrl: "https://api.neutron.quokkastake.io/cosmos/bank/v1beta1/balances",
	},
	Neptune: {
		Protocol:          Neptune,
		PoolInfoUrl:       "https://injective-api.polkachu.com/cosmwasm/wasm/v1/contract",
		AssetListURL:      "https://chains.cosmos.directory/injective",
		AddressBalanceUrl: "",
	},
	Ux: {
		Protocol:          Ux,
		PoolInfoUrl:       "https://umee-api.polkachu.com/umee",
		AssetListURL:      "https://chains.cosmos.directory/umee",
		AddressBalanceUrl: "",
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
	Pryzm: {
		Protocol:          Pryzm,
		PoolInfoUrl:       "",
		AssetListURL:      "",
		AddressBalanceUrl: "",
	},
}

// map of bid ID to its position config
var bidMap = map[int]BidPositionConfig{
	0: {
		InitialAllocation: 10557,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: Margined},
		},
	},
	1: {
		InitialAllocation: 10000,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: Demex},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 1, 24, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 10078,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	2: {
		InitialAllocation: 18000,
		Venues: []VenuePositionConfig{
			NeptuneVenuePositionConfig{
				Denom:        INJECTIVE_ATOM,
				Address:      "inj1up8gwq9utn4mmegfn289l5ddsgkmktncrxxe9z",
				ActiveShares: 0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 1, 18, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 18405,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	3: {
		InitialAllocation: 50000,
		Venues: []VenuePositionConfig{
			AstroportVenuePositionConfig{
				Protocol:         AstroportNeutron,
				PoolAddress:      "neutron1yem82r0wf837lfkwvcu2zxlyds5qrzwkz8alvmg0apyrjthk64gqeq2e98",
				IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqgknctjyq6m3tch",
				Address:          "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
				ActiveShares:     0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 1, 17, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 0,
				WithdrawnShares: 0,
				CompoundedBidId: 11,
			},
		},
	},
	4: {
		InitialAllocation: 36093,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: Shade},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 1, 18, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 0,
				WithdrawnShares: 0,
				CompoundedBidId: 12,
			},
		},
	},
	5: {
		InitialAllocation: 10000,
		Venues: []VenuePositionConfig{
			NolusVenuePositionConfig{
				PoolContractAddress: "nolus1jufcaqm6657xmfltdezzz85quz92rmtd88jk5x0hq9zqseem32ysjdm990",
				PoolContractToken:   NOLUS_ST_ATOM,
				Address:             "nolus1u74s6nuqgulf9kuezjt9q8r8ghx0kcvcl96fx63nk29df25n2u5swmz3g6",
				ActiveShares:        0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 1, 18, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 10044,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	6: {
		InitialAllocation: 3143,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: WhiteWhale},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 1, 18, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 0,
				WithdrawnShares: 0,
				CompoundedBidId: 14,
			},
		},
	},
	7: {
		InitialAllocation: 17912,
		Venues: []VenuePositionConfig{
			AstroportVenuePositionConfig{
				Protocol:         AstroportTerra,
				PoolAddress:      "terra1f9vmtntpjmkyhkxtlc49jcq6cv8rfz0kr06zv6efdtdgae4m9y9qlzm36t",
				IncentiveAddress: "terra1eywh4av8sln6r45pxq45ltj798htfy0cfcf7fy3pxc2gcv6uc07se4ch9x",
				Address:          "terra12wq57ea7m7m8wx4qhsj04fyc78pv2n3h888vfzuv7n7k7qlq2dyssuyf8h",
				ActiveShares:     0,
			},
			AstroportVenuePositionConfig{
				Protocol:         AstroportTerra,
				PoolAddress:      "terra1a0h6vrzkztjystg8sd949qyrc6mw9gzxk2870cr2mukg53uzgvqs46qul9",
				IncentiveAddress: "terra1eywh4av8sln6r45pxq45ltj798htfy0cfcf7fy3pxc2gcv6uc07se4ch9x",
				Address:          "terra12wq57ea7m7m8wx4qhsj04fyc78pv2n3h888vfzuv7n7k7qlq2dyssuyf8h",
				ActiveShares:     0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 1, 18, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 0,
				WithdrawnShares: 0,
				CompoundedBidId: 15,
			},
		},
	},
	// round 2 starts here
	11: {
		InitialAllocation: 81000,
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
				ActiveShares:     0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 4, 20, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 85077,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	12: {
		InitialAllocation: 33953,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: Shade},
		},
	},
	14: {
		InitialAllocation: 10000,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: WhiteWhale},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 7, 12, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 12576,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	15: {
		InitialAllocation: 26000,
		Venues: []VenuePositionConfig{
			AstroportVenuePositionConfig{
				Protocol:         AstroportTerra,
				PoolAddress:      "terra1f9vmtntpjmkyhkxtlc49jcq6cv8rfz0kr06zv6efdtdgae4m9y9qlzm36t",
				IncentiveAddress: "terra1eywh4av8sln6r45pxq45ltj798htfy0cfcf7fy3pxc2gcv6uc07se4ch9x",
				Address:          "terra12wq57ea7m7m8wx4qhsj04fyc78pv2n3h888vfzuv7n7k7qlq2dyssuyf8h",
				ActiveShares:     0,
			},
			AstroportVenuePositionConfig{
				Protocol:         AstroportTerra,
				PoolAddress:      "terra1a0h6vrzkztjystg8sd949qyrc6mw9gzxk2870cr2mukg53uzgvqs46qul9",
				IncentiveAddress: "terra1eywh4av8sln6r45pxq45ltj798htfy0cfcf7fy3pxc2gcv6uc07se4ch9x",
				Address:          "terra12wq57ea7m7m8wx4qhsj04fyc78pv2n3h888vfzuv7n7k7qlq2dyssuyf8h",
				ActiveShares:     0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 4, 15, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 0,
				WithdrawnShares: 0,
				CompoundedBidId: 32,
			},
		},
	},
	16: {
		InitialAllocation: 42000,
		Venues: []VenuePositionConfig{
			MarsVenuePositionConfig{
				CreditAccountID: "2533",
				DepositedDenom:  NEUTRON_ATOM,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 4, 15, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 42455,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	17: {
		InitialAllocation: 51000,
		Venues: []VenuePositionConfig{
			OsmosisVenuePositionConfig{
				PoolID:     "1283",
				Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
				PositionID: "11124334",
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 4, 20, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 52300,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	// round 3 starts here
	18: {
		InitialAllocation: 45585,
		Venues: []VenuePositionConfig{
			OsmosisVenuePositionConfig{
				PoolID:     "1283",
				Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
				PositionID: "11701290",
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 4, 13, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 46203,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	22: {
		InitialAllocation: 10000,
		Venues: []VenuePositionConfig{
			AstroportVenuePositionConfig{
				Protocol:         AstroportNeutron,
				PoolAddress:      "neutron14y0xyavpf5xznw56u3xml9f2jmx8ruk3y8f5e6zzkd9mhmcps3fs59g4vt",
				IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqg4gknctjyq6m3tch",
				Address:          "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
				ActiveShares:     0,
			},
			AstroportVenuePositionConfig{
				Protocol:         AstroportNeutron,
				PoolAddress:      "neutron1w8vmg3zwyh62edp7uxpaw90447da9zzlv0kqh2ajye6a6mseg06qseyv5m",
				IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqg4gknctjyq6m3tch",
				Address:          "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
				ActiveShares:     0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 5, 13, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 0,
				WithdrawnShares: 0,
				CompoundedBidId: 53,
			},
		},
	},
	23: {
		InitialAllocation: 22340,
		Venues: []VenuePositionConfig{
			NolusVenuePositionConfig{
				PoolContractAddress: "nolus1u0zt8x3mkver0447glfupz9lz6wnt62j70p5fhhtu3fr46gcdd9s5dz9l6",
				PoolContractToken:   NOLUS_ATOM,
				Address:             "nolus1u74s6nuqgulf9kuezjt9q8r8ghx0kcvcl96fx63nk29df25n2u5swmz3g6",
				ActiveShares:        0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 5, 13, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 22340,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	24: {
		InitialAllocation: 43962,
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
				ActiveShares:     0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 5, 20, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 44757,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	// round 4 starts here
	25: {
		InitialAllocation: 170000,
		Venues: []VenuePositionConfig{
			OsmosisVenuePositionConfig{
				PoolID:     "1283",
				Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
				PositionID: "12515115",
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 178563,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	27: {
		InitialAllocation: 10000,
		Venues: []VenuePositionConfig{
			NeptuneVenuePositionConfig{
				Denom:        INJECTIVE_ATOM,
				Address:      "inj1up8gwq9utn4mmegfn289l5ddsgkmktncrxxe9z",
				ActiveShares: 0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 6, 23, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 10168,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	// round 5 starts here
	31: {
		InitialAllocation: 153000,
		Venues: []VenuePositionConfig{
			MarsVenuePositionConfig{
				CreditAccountID: "4068",
				DepositedDenom:  NEUTRON_ATOM,
			},
		},
	},
	32: {
		InitialAllocation: 48000,
		Venues: []VenuePositionConfig{
			AstroportVenuePositionConfig{
				Protocol:         AstroportTerra,
				PoolAddress:      "terra1f9vmtntpjmkyhkxtlc49jcq6cv8rfz0kr06zv6efdtdgae4m9y9qlzm36t",
				IncentiveAddress: "terra1eywh4av8sln6r45pxq45ltj798htfy0cfcf7fy3pxc2gcv6uc07se4ch9x",
				Address:          "terra12wq57ea7m7m8wx4qhsj04fyc78pv2n3h888vfzuv7n7k7qlq2dyssuyf8h",
				ActiveShares:     0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 7, 21, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 48717,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	33: {
		InitialAllocation: 108000,
		Venues: []VenuePositionConfig{
			OsmosisVenuePositionConfig{
				PoolID:     "1283",
				Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
				PositionID: "13333641",
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 7, 23, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 111166,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	36: {
		InitialAllocation: 30000,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: Margined},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 7, 23, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 30663,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	38: {
		InitialAllocation: 48000,
		Venues: []VenuePositionConfig{
			AstroportVenuePositionConfig{
				Protocol:         AstroportNeutron,
				PoolAddress:      "neutron1yem82r0wf837lfkwvcu2zxlyds5qrzwkz8alvmg0apyrjthk64gqeq2e98",
				IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqgknctjyq6m3tch",
				Address:          "neutron1jdryd7eza5g68s9rzeqhckpsqx0dr8wcncpkq57pwdyvm3uvwhcqxp2865", //valence acc
				ActiveShares:     0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 7, 7, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 50743,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	39: {
		InitialAllocation: 37050,
		Venues: []VenuePositionConfig{
			NolusVenuePositionConfig{
				PoolContractAddress: "nolus1ueytzwqyadm6r0z8ajse7g6gzum4w3vv04qazctf8ugqrrej6n4sq027cf",
				PoolContractToken:   NOLUS_USDC,
				Address:             "nolus1u74s6nuqgulf9kuezjt9q8r8ghx0kcvcl96fx63nk29df25n2u5swmz3g6",
				ActiveShares:        0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 5, 25, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 37647,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	40: {
		InitialAllocation: 19950,
		Venues: []VenuePositionConfig{
			ElysVenuePositionConfig{
				Address:      "elys14crljzq0qmgaqdcpr69sna3z0r83u29srdxv8qvnfq9n7uj4kgtqg4quae",
				PoolId:       "32767",
				ActiveShares: 0,
				PoolType:     Stablestake,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 6, 13, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 0,
				WithdrawnShares: 0,
				CompoundedBidId: 54,
			},
		},
	},
	// round 6 starts here
	41: {
		InitialAllocation: 224000,
		Venues: []VenuePositionConfig{
			MarsVenuePositionConfig{
				CreditAccountID: "4612",
				DepositedDenom:  NEUTRON_ATOM,
			},
		},
	},
	42: {
		InitialAllocation: 40000,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: Shade},
		},
	},
	43: {
		InitialAllocation: 112000,
		Venues: []VenuePositionConfig{
			OsmosisVenuePositionConfig{
				PoolID:     "1283",
				Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
				PositionID: "14010188",
			},
		},
	},
	45: {
		InitialAllocation: 172000,
		Venues: []VenuePositionConfig{
			ElysVenuePositionConfig{
				Address:      "elys14crljzq0qmgaqdcpr69sna3z0r83u29srdxv8qvnfq9n7uj4kgtqg4quae",
				PoolId:       "32768",
				ActiveShares: 171724645382,
				PoolType:     Stablestake,
			},
		},
	},
	48: {
		InitialAllocation: 10000,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: Pryzm},
		},
	},
	// round 7 starts here
	50: {
		InitialAllocation: 367300,
		Venues: []VenuePositionConfig{
			OsmosisVenuePositionConfig{
				PoolID:     "1283",
				Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
				PositionID: "14570507",
			},
			OsmosisVenuePositionConfig{
				PoolID:     "1283",
				Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
				PositionID: "14691901",
			},
		},
	},
	51: {
		InitialAllocation: 78000,
		Venues: []VenuePositionConfig{
			DualityVenuePositionConfig{
				PoolAddress:  "neutron18ua532r8lpy8scvysrgcjneyrwuj4x0ne4t2azphxksya596l4cq23lkp9",
				Address:      "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
				ActiveShares: 330342489391671,
			},
		},
	},
	53: {
		InitialAllocation: 10000,
		Venues: []VenuePositionConfig{
			AstroportVenuePositionConfig{
				Protocol:         AstroportNeutron,
				PoolAddress:      "neutron14y0xyavpf5xznw56u3xml9f2jmx8ruk3y8f5e6zzkd9mhmcps3fs59g4vt",
				IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqg4gknctjyq6m3tch",
				Address:          "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
				ActiveShares:     0,
			},
			AstroportVenuePositionConfig{
				Protocol:         AstroportNeutron,
				PoolAddress:      "neutron1w8vmg3zwyh62edp7uxpaw90447da9zzlv0kqh2ajye6a6mseg06qseyv5m",
				IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqg4gknctjyq6m3tch",
				Address:          "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
				ActiveShares:     0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 7, 6, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 0,
				WithdrawnShares: 0,
				CompoundedBidId: 62,
			},
		},
	},
	54: {
		InitialAllocation: 26790,
		Venues: []VenuePositionConfig{
			ElysVenuePositionConfig{
				Address:      "elys14crljzq0qmgaqdcpr69sna3z0r83u29srdxv8qvnfq9n7uj4kgtqg4quae",
				PoolId:       "32767",
				ActiveShares: 0,
				PoolType:     Stablestake,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 7, 11, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 26903,
				WithdrawnShares: 0,
				CompoundedBidId: 0,
			},
		},
	},
	55: {
		InitialAllocation: 42000,
		Venues: []VenuePositionConfig{
			AstroportVenuePositionConfig{
				Protocol:         AstroportTerra,
				PoolAddress:      "terra1a0h6vrzkztjystg8sd949qyrc6mw9gzxk2870cr2mukg53uzgvqs46qul9",
				IncentiveAddress: "terra1eywh4av8sln6r45pxq45ltj798htfy0cfcf7fy3pxc2gcv6uc07se4ch9x",
				Address:          "terra12wq57ea7m7m8wx4qhsj04fyc78pv2n3h888vfzuv7n7k7qlq2dyssuyf8h",
				ActiveShares:     19264866037,
			},
		},
	},
	// round 8 starts here
	57: {
		InitialAllocation: 31920,
		Venues: []VenuePositionConfig{
			NolusVenuePositionConfig{
				PoolContractAddress: "nolus1ueytzwqyadm6r0z8ajse7g6gzum4w3vv04qazctf8ugqrrej6n4sq027cf",
				PoolContractToken:   NOLUS_USDC,
				Address:             "nolus1u74s6nuqgulf9kuezjt9q8r8ghx0kcvcl96fx63nk29df25n2u5swmz3g6",
				ActiveShares:        28988735638,
			},
		},
	},
	58: {
		InitialAllocation: 101586,
		Venues: []VenuePositionConfig{
			OsmosisVenuePositionConfig{
				PoolID:     "1283",
				Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
				PositionID: "14924293",
			},
		},
	},
	59: {
		InitialAllocation: 66020,
		Venues: []VenuePositionConfig{
			AstroportVenuePositionConfig{
				Protocol:         AstroportTerra,
				PoolAddress:      "terra1a0h6vrzkztjystg8sd949qyrc6mw9gzxk2870cr2mukg53uzgvqs46qul9",
				IncentiveAddress: "terra1eywh4av8sln6r45pxq45ltj798htfy0cfcf7fy3pxc2gcv6uc07se4ch9x",
				Address:          "terra12wq57ea7m7m8wx4qhsj04fyc78pv2n3h888vfzuv7n7k7qlq2dyssuyf8h",
				ActiveShares:     30349183715,
			},
		},
	},
	60: {
		InitialAllocation: 198000,
		Venues: []VenuePositionConfig{
			MarsVenuePositionConfig{
				CreditAccountID: "5054",
				DepositedDenom:  NEUTRON_ATOM,
			},
		},
	},
	62: {
		InitialAllocation: 10000,
		Venues: []VenuePositionConfig{
			AstroportVenuePositionConfig{
				Protocol:         AstroportNeutron,
				PoolAddress:      "neutron14y0xyavpf5xznw56u3xml9f2jmx8ruk3y8f5e6zzkd9mhmcps3fs59g4vt",
				IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqg4gknctjyq6m3tch",
				Address:          "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
				ActiveShares:     0,
			},
			AstroportVenuePositionConfig{
				Protocol:         AstroportNeutron,
				PoolAddress:      "neutron1w8vmg3zwyh62edp7uxpaw90447da9zzlv0kqh2ajye6a6mseg06qseyv5m",
				IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqg4gknctjyq6m3tch",
				Address:          "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
				ActiveShares:     0,
			},
		},
		Withdrawals: []Withdrawal{
			{
				Date:            time.Date(2025, 8, 6, 0, 0, 0, 0, time.UTC),
				WithdrawnAmount: 0,
				WithdrawnShares: 0,
				CompoundedBidId: 78,
			},
		},
	},
	65: {
		InitialAllocation: 8755, // 2888 ATOM and 25084 USDC ~ 8.5k ATOM
		Venues: []VenuePositionConfig{
			ElysVenuePositionConfig{
				Address:      "elys14crljzq0qmgaqdcpr69sna3z0r83u29srdxv8qvnfq9n7uj4kgtqg4quae",
				PoolId:       "1",
				ActiveShares: 52305580544014690236115,
				PoolType:     AMM,
			},
		},
	},
	67: {
		InitialAllocation: 30000,
		Venues: []VenuePositionConfig{
			UxVenuePositionConfig{
				Address: "umee18zw3ud29vtxqvlljrnnexphtn62yccc700lek432cy9ngv4n4kgqupkr02",
				Denom:   UX_ATOM,
			},
		},
	},
	// round 9 starts here
	70: {
		InitialAllocation: 36000,
		Venues: []VenuePositionConfig{
			DualityVenuePositionConfig{
				PoolAddress:  "neutron18ua532r8lpy8scvysrgcjneyrwuj4x0ne4t2azphxksya596l4cq23lkp9",
				Address:      "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
				ActiveShares: 147306958149831,
			},
		},
	},
	71: {
		InitialAllocation: 144000,
		Venues: []VenuePositionConfig{
			MarsVenuePositionConfig{
				CreditAccountID: "5189",
				DepositedDenom:  NEUTRON_ATOM,
			},
		},
	},
	72: {
		InitialAllocation: 13800,
		Venues: []VenuePositionConfig{
			NeptuneVenuePositionConfig{
				Denom:        INJECTIVE_ATOM,
				Address:      "inj1up8gwq9utn4mmegfn289l5ddsgkmktncrxxe9z",
				ActiveShares: 12968316918,
			},
		},
	},
	77: {
		InitialAllocation: 749, // 749 atom, 609302 arch
		Venues: []VenuePositionConfig{
			OsmosisVenuePositionConfig{
				PoolID:     "3111",
				Address:    "osmo16cuqr48efufwf78gfk2yfjs08av5levpe4ge2zynrkrxu98gn2zs7r9jh4", // vortex contract
				PositionID: "14958520",
			},
		},
	},
	//todo: check what happened with compounding on 78
	// 78: {
	// 	InitialAllocation: 10000,
	// 	Venues: []VenuePositionConfig{
	// 		AstroportVenuePositionConfig{
	// 			Protocol:         AstroportNeutron,
	// 			PoolAddress:      "neutron14y0xyavpf5xznw56u3xml9f2jmx8ruk3y8f5e6zzkd9mhmcps3fs59g4vt",
	// 			IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqg4gknctjyq6m3tch",
	// 			Address:          "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
	// 			ActiveShares:     0,
	// 		},
	// 		AstroportVenuePositionConfig{
	// 			Protocol:         AstroportNeutron,
	// 			PoolAddress:      "neutron1w8vmg3zwyh62edp7uxpaw90447da9zzlv0kqh2ajye6a6mseg06qseyv5m",
	// 			IncentiveAddress: "neutron173fd8wpfzyqnfnpwq2zhtgdstujrjz2wkprkjfr6gqg4gknctjyq6m3tch",
	// 			Address:          "neutron1w7f40hgfc505a2wnjsl5pg35yl8qpawv48w5yekax4xj2m43j09s5fa44f",
	// 			ActiveShares:     0,
	// 		},
	// 	},
	// },
	79: {
		InitialAllocation: 46900,
		Venues: []VenuePositionConfig{
			OsmosisVenuePositionConfig{
				PoolID:     "1283",
				Address:    "osmo1cuwe7dzgpemwxqzpkhyjwfeev2hcgd9de8xp566hrly6wtpcrc7qgp9jdx",
				PositionID: "14950170",
			},
		},
	},
	81: {
		InitialAllocation: 10000,
		Venues: []VenuePositionConfig{
			MissingVenuePositionConfig{Protocol: Pryzm},
		},
	},
}
