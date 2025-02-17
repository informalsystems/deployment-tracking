package main

// Protocol type enum
type Protocol string

const (
	Osmosis   Protocol = "osmosis"
	Astroport Protocol = "astroport"
)

// Core data structures
type ProtocolConfig struct {
	Protocol     Protocol
	PoolID       string
	LCDEndpoint  string
	AssetListURL string
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
	ComputeTVL() (Holdings, error)
	ComputeAddressPrincipalHoldings(address string) (Holdings, error)
	ComputeAddressRewardHoldings(address string) (Holdings, error)
	ComputeVenueHoldings() (VenueHoldings, error)
}
