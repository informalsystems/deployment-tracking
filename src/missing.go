package main

import "fmt"

type MissingVenuePositionConfig struct {
	Protocol Protocol
}

func (venueConfig MissingVenuePositionConfig) GetProtocol() Protocol {
	return venueConfig.Protocol
}

func (venueConfig MissingVenuePositionConfig) GetPoolID() string {
	return ""
}

func (venueConfig MissingVenuePositionConfig) GetAddress() string {
	return ""
}

type MissingPosition struct {
	protocolConfig      ProtocolConfig
	venuePositionConfig MissingVenuePositionConfig
}

func NewMissingPosition(config ProtocolConfig, venuePositionConfig VenuePositionConfig) (*MissingPosition, error) {
	missingVenuePositionConfig, ok := venuePositionConfig.(MissingVenuePositionConfig)
	if !ok {
		return nil, fmt.Errorf("venuePositionConfig must be of MissingVenuePositionConfig type")
	}

	return &MissingPosition{protocolConfig: config, venuePositionConfig: missingVenuePositionConfig}, nil
}

func (p MissingPosition) ComputeTVL(assetData *ChainInfo) (*Holdings, error) {
	return nil, nil
}

func (p MissingPosition) ComputeAddressPrincipalHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	return nil, nil
}

func (p MissingPosition) ComputeAddressRewardHoldings(assetData *ChainInfo, address string) (*Holdings, error) {
	return nil, nil
}
