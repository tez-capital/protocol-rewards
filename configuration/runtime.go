package configuration

import (
	"os"

	"github.com/hjson/hjson-go/v4"
)

type DatabaseConfiguration struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

func (dc *DatabaseConfiguration) Unwrap() (host string, port string, user string, pass string, database string) {
	return dc.Host, dc.Port, dc.User, dc.Password, dc.Database
}

type Runtime struct {
	Database  DatabaseConfiguration `json:"database"`
	Listen    []string              `json:"listen"`
	Providers []string              `json:"providers"`
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
