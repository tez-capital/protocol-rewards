package tezos

import (
	"encoding/json"
	"errors"
)

var (
	ErrInvalidNumberOfRpcs = errors.New("invalid number of rpcs")
)

type TezosRpcConfiguration struct {
	Providers               []string `json:"providers"`
	NumberOfActiveProviders int      `json:"number_of_active_providers"`
}

func LoadTezosConfiguration(content json.RawMessage) (*TezosRpcConfiguration, error) {
	var config TezosRpcConfiguration
	err := json.Unmarshal(content, &config)
	if err != nil {
		return nil, err
	}

	if config.NumberOfActiveProviders == 0 {
		config.NumberOfActiveProviders = 1
	}

	if len(config.Providers) < config.NumberOfActiveProviders {
		config.NumberOfActiveProviders = len(config.Providers)
	}

	if config.NumberOfActiveProviders == 0 {
		return nil, ErrInvalidNumberOfRpcs
	}

	return &config, nil
}
