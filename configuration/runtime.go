package configuration

import (
	"log/slog"
	"os"

	"github.com/hjson/hjson-go/v4"
	"github.com/joho/godotenv"
	"github.com/tez-capital/ogun/constants"
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
	Database      DatabaseConfiguration `json:"database"`
	Listen        string                `json:"listen"`
	PrivateListen string                `json:"private_listen"`
	Providers     []string              `json:"providers"`
	LogLevel      slog.Level            `json:"-"`
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

	if err = godotenv.Load(); err != nil {
		slog.Debug("error loading .env file", "error", err)
	}

	runtimeConfig.LogLevel = getLogLevel(os.Getenv(constants.LOG_LEVEL))

	return &runtimeConfig, nil
}

func getLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
