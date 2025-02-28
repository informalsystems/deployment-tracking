package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/patrickmn/go-cache"
)

// Constants
const (
	Debug = true
	BidId = "7.statom"
)

// Global cache instance (cache duration: 30 minutes)
var resultCache *cache.Cache

// --- Business Logic Layer ---

// computeHoldings computes the holdings for a given bid.
func computeHoldings(bidId string) (*VenueHoldings, error) {
	// get the config for the bid
	bidConfig, ok := bidMap[bidId]
	if !ok {
		return nil, fmt.Errorf("bid not found: %s", bidId)
	}

	// if there is a result not older than 30 minutes, return it
	if cached, found := resultCache.Get(bidId); found {
		return cached.(*VenueHoldings), nil
	}

	// get the protocol config
	protocolConfig := protocolConfigMap[bidConfig.GetProtocol()]

	// construct the protocol
	protocol, err := NewDexProtocolFromConfig(protocolConfig, bidConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating protocol: %w", err)
	}

	assetData, err := fetchAssetList(protocolConfig.AssetListURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching asset list: %w", err)
	}

	tvl, err := protocol.ComputeTVL(assetData)
	if err != nil {
		return nil, fmt.Errorf("error computing TVL: %w", err)
	}

	addressHoldings, err := protocol.ComputeAddressPrincipalHoldings(assetData, bidConfig.GetAddress())
	if err != nil {
		return nil, fmt.Errorf("error computing address principal holdings: %w", err)
	}

	rewardHoldings, err := protocol.ComputeAddressRewardHoldings(assetData, bidConfig.GetAddress())
	if err != nil {
		return nil, fmt.Errorf("error computing address reward holdings: %w", err)
	}

	venueHoldings := VenueHoldings{
		VenueTotal:       *tvl,
		AddressPrincipal: *addressHoldings,
		AddressRewards:   *rewardHoldings,
	}

	// Cache the JSON result for 30 minutes.
	resultCache.Set(bidId, &venueHoldings, cache.DefaultExpiration)

	return &venueHoldings, nil
}

// --- HTTP Handler Layer ---

// holdingsHandler serves the computed holdings data.
// It first checks the cache and, if a valid cached result exists,
// returns that. Otherwise, it computes the result, caches it for 30 minutes, and returns it.
func holdingsHandler(w http.ResponseWriter, r *http.Request) {
	bidId := mux.Vars(r)["bid_id"]

	// If no Bid ID is provided, return holdings of all bids
	if bidId == "" {
		allHoldings := make([]BidHoldings, 0, len(bidMap))

		for bidId := range bidMap {
			holdings, err := computeHoldings(bidId)
			if err != nil {
				debugLog(fmt.Sprintf("failed to compute holdings for bid ID: %s", bidId), nil)
				holdings = nil
			}

			allHoldings = append(allHoldings, BidHoldings{BidId: bidId, Holdings: holdings})
		}

		jsonData, err := json.MarshalIndent(allHoldings, "", "  ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)

		return
	}

	// Compute holdings.
	holdings, err := computeHoldings(bidId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Marshal holdings to JSON.
	jsonData, err := json.MarshalIndent(holdings, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

// --- Main / Server Bootstrap ---

func main() {
	// Define the --debug flag.
	debug := flag.Bool("debug", false, "Run the endpoint once for testing")
	flag.Parse()

	// If the --debug flag is provided, run the endpoint logic once and exit.
	if *debug {
		holdings, err := computeHoldings(BidId)
		if err != nil {
			log.Fatalf("Error computing holdings: %v", err)
		}
		jsonData, err := json.MarshalIndent(holdings, "", "  ")
		if err != nil {
			log.Fatalf("Error marshalling holdings: %v", err)
		}
		fmt.Println(string(jsonData))
		return
	}

	// Initialize the in-memory cache with a 30-minute expiration and a 10-minute cleanup interval.
	resultCache = cache.New(30*time.Minute, 10*time.Minute)

	router := mux.NewRouter()

	// Register the holdings endpoints.
	router.HandleFunc("/holdings/", holdingsHandler)
	router.HandleFunc("/holdings/{bid_id}", holdingsHandler)

	// Start the HTTP server.
	port := ":8080"
	log.Printf("Server is running on port %s", port)
	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
