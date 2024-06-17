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
	Listen        string                `json:"-"`
	PrivateListen string                `json:"-"`
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
		slog.Info("error loading .env file, loading env variables directly from environment or if not found load the defaults", "error", err)
	}

	runtimeConfig.LogLevel = GetLogLevel(os.Getenv(constants.LOG_LEVEL))
	runtimeConfig.Listen = os.Getenv(constants.LISTEN)
	if runtimeConfig.Listen == "" {
		runtimeConfig.Listen = constants.LISTEN_DEFAULT
	}

	runtimeConfig.PrivateListen = os.Getenv(constants.PRIVATE_LISTEN)
	if runtimeConfig.PrivateListen == "" {
		runtimeConfig.PrivateListen = constants.PRIVATE_LISTEN_DEFAULT
	}

	return &runtimeConfig, nil
}

func GetLogLevel(level string) slog.Level {
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
