package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/patrickmn/go-cache"
)

// Constants
const (
	Debug = true
	BidId = 26
)

// Global cache instance (cache duration: 30 minutes)
var resultCache *cache.Cache

// --- Business Logic Layer ---

// computeHoldings computes the holdings for a given bid.
func computeHoldings(bidId int) ([]VenueHoldings, error) {
	// get the config for the bid
	bidConfig, ok := bidMap[bidId]
	if !ok {
		return nil, fmt.Errorf("bid not found: %d", bidId)
	}

	// if there is a result not older than 30 minutes, return it
	if cached, found := resultCache.Get(strconv.Itoa(bidId)); found {
		return cached.([]VenueHoldings), nil
	}

	bidHoldings := make([]VenueHoldings, 0, len(bidConfig.Venues))

	for _, venueConfig := range bidConfig.Venues {
		// get the protocol config
		protocolConfig := protocolConfigMap[venueConfig.GetProtocol()]

		// construct the protocol
		protocol, err := NewDexProtocolFromConfig(protocolConfig, venueConfig)
		if err != nil {
			return nil, fmt.Errorf("error creating protocol: %w", err)
		}

		if _, ok := protocol.(*MissingPosition); ok {
			venueHoldings := VenueHoldings{
				InfoMissing:      true,
				Protocol:         venueConfig.GetProtocol(),
				VenueTotal:       nil,
				AddressPrincipal: nil,
				AddressRewards:   nil,
			}

			bidHoldings = append(bidHoldings, venueHoldings)

			continue
		}

		assetData, err := fetchAssetList(protocolConfig.AssetListURL)
		if err != nil {
			return nil, fmt.Errorf("error fetching asset list: %w", err)
		}

		tvl, err := protocol.ComputeTVL(assetData)
		if err != nil {
			return nil, fmt.Errorf("error computing TVL: %w", err)
		}

		addressHoldings, err := protocol.ComputeAddressPrincipalHoldings(assetData, venueConfig.GetAddress())
		if err != nil {
			return nil, fmt.Errorf("error computing address principal holdings: %w", err)
		}

		rewardHoldings, err := protocol.ComputeAddressRewardHoldings(assetData, venueConfig.GetAddress())
		if err != nil {
			return nil, fmt.Errorf("error computing address reward holdings: %w", err)
		}

		venueHoldings := VenueHoldings{
			InfoMissing:      false,
			Protocol:         venueConfig.GetProtocol(),
			VenueTotal:       tvl,
			AddressPrincipal: addressHoldings,
			AddressRewards:   rewardHoldings,
		}

		bidHoldings = append(bidHoldings, venueHoldings)
	}

	// Cache the JSON result for 30 minutes.
	resultCache.Set(strconv.Itoa(bidId), bidHoldings, cache.DefaultExpiration)

	return bidHoldings, nil
}

// --- HTTP Handler Layer ---

// holdingsHandler serves the computed holdings data.
// It first checks the cache and, if a valid cached result exists,
// returns that. Otherwise, it computes the result, caches it for 30 minutes, and returns it.
func holdingsHandler(w http.ResponseWriter, r *http.Request) {
	bidIdStr := mux.Vars(r)["bid_id"]

	// If no Bid ID is provided, return holdings of all bids
	if bidIdStr == "" {
		allHoldings := make([]BidHoldings, 0, len(bidMap))

		for bidId, bidConfig := range bidMap {
			holdings, err := computeHoldings(bidId)
			if err != nil {
				debugLog(fmt.Sprintf("failed to compute holdings for bid ID: %d", bidId), nil)
				holdings = nil
			}

			allHoldings = append(allHoldings, BidHoldings{BidId: bidId, InitialAtomAllocation: bidConfig.InitialAtomAllocation, Holdings: holdings})
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

	bidId, err := strconv.Atoi(bidIdStr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

// experimentalHandler serves data about experimental deployments
func experimentalHandler(w http.ResponseWriter, r *http.Request) {
	// Get experimental ID from query params
	experimentalIdStr := r.URL.Query().Get("id")

	// Get asset data for computing holdings
	assetData, err := fetchAssetList("https://chains.cosmos.directory/osmosis") // Using Osmosis for now
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching asset list: %v", err), http.StatusInternalServerError)
		return
	}

	// If no ID provided, return all experimental deployments
	if experimentalIdStr == "" {
		allDeployments := make([]ExperimentalDeploymentResponse, 0, len(experimentalMap))
		for _, deployment := range experimentalMap {
			// Compute current holdings for each deployment
			currentHoldings, err := deployment.Querier.GetCurrentAddressHoldings(assetData)
			if err != nil {
				debugLog(fmt.Sprintf("Error computing holdings for deployment %d: %v", deployment.ExperimentalId, err), nil)
				currentHoldings = nil
			}

			response := ExperimentalDeploymentResponse{
				ExperimentalId:         deployment.ExperimentalId,
				Name:                   deployment.Name,
				Description:            deployment.Description,
				Logo:                   deployment.Logo,
				StartTimestamp:         deployment.StartTimestamp,
				EndTimestamp:           deployment.EndTimestamp,
				InitialAddressHoldings: deployment.InitialAddressHoldings,
				CurrentAddressHoldings: currentHoldings,
			}
			allDeployments = append(allDeployments, response)
		}

		jsonData, err := json.MarshalIndent(allDeployments, "", "  ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)
		return
	}

	// Parse experimental ID
	experimentalId, err := strconv.Atoi(experimentalIdStr)
	if err != nil {
		http.Error(w, "Invalid experimental ID", http.StatusBadRequest)
		return
	}

	// Get deployment config
	deployment, ok := experimentalMap[experimentalId]
	if !ok {
		http.Error(w, "Experimental deployment not found", http.StatusNotFound)
		return
	}

	// Compute holdings
	currentHoldings, err := deployment.Querier.GetCurrentAddressHoldings(assetData)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error computing current holdings: %v", err), http.StatusInternalServerError)
		return
	}

	// Create response
	response := ExperimentalDeploymentResponse{
		ExperimentalId:         deployment.ExperimentalId,
		Name:                   deployment.Name,
		Description:            deployment.Description,
		Logo:                   deployment.Logo,
		StartTimestamp:         deployment.StartTimestamp,
		EndTimestamp:           deployment.EndTimestamp,
		InitialAddressHoldings: deployment.InitialAddressHoldings,
		CurrentAddressHoldings: currentHoldings,
	}

	// Return response
	jsonData, err := json.MarshalIndent(response, "", "  ")
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

	// Initialize the in-memory cache with a 30-minute expiration and a 10-minute cleanup interval.
	resultCache = cache.New(30*time.Minute, 10*time.Minute)

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

	router := mux.NewRouter()

	// Register the endpoints.
	router.HandleFunc("/holdings/", holdingsHandler)
	router.HandleFunc("/holdings/{bid_id}", holdingsHandler)
	router.HandleFunc("/experimental", experimentalHandler)

	// Start the HTTP server.
	port := ":8080"
	log.Printf("Server is running on port %s", port)
	if err := http.ListenAndServe(port, router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
