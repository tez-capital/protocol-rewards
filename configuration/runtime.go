package configuration

import (
	"encoding/json"
	"os"

	"github.com/hjson/hjson-go/v4"
	"github.com/tez-capital/ogun/configuration/tezos"
	"github.com/tez-capital/ogun/constants"
)

type DatabaseConfiguration struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

func (dc *DatabaseConfiguration) Unwrap() (string, string, string, string, string) {
	return dc.Host, dc.Port, dc.User, dc.Password, dc.Database
}

type EventSelector struct {
	Source string `json:"source"`
	Event  string `json:"event"`
}

type ConnectConfiguration struct {
	Forward []EventSelector `json:"forward"`
	Consume []EventSelector `json:"consume"`
}

type Runtime struct {
	Environment string                     `json:"environment"`
	Subsystems  map[string]json.RawMessage `json:"subsystems"`
	Database    DatabaseConfiguration      `json:"database"`
	Listen      []string                   `json:"listen"`
	BatchSize   int                        `json:"batch_size"`
}

func LoadConfiguration(path string) (*Runtime, error) {
	configBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse the config file to the RuntimeConfiguration struct
	var runtimeConfig Runtime
	err = hjson.Unmarshal(configBytes, &runtimeConfig)
	if err != nil {
		return nil, err
	}

	return &runtimeConfig, nil
}

func (r *Runtime) GetTezosConfiguration() (*tezos.TezosRpcConfiguration, error) {
	if tezosConfiguration, ok := r.Subsystems["tezos"]; ok {
		return tezos.LoadTezosConfiguration(tezosConfiguration)
	}
	return nil, constants.ErrSubsystemNotFound
}
