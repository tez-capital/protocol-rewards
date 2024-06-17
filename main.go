package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/tez-capital/ogun/api"
	"github.com/tez-capital/ogun/configuration"
	"github.com/tez-capital/ogun/core"
)

func main() {
	configPath := flag.String("config", "config.hjson", "path to the configuration file")
	logLevel := flag.String("log", "", "set the desired log level")
	isTest := flag.Bool("test", false, "run the test")

	ctx, cancel := context.WithCancel(context.Background())

	flag.Parse()

	config, err := configuration.LoadConfiguration(*configPath)
	if err != nil {
		panic(err)
	}

	if *logLevel != "" {
		config.LogLevel = configuration.GetLogLevel(*logLevel)
	}

	engine, err := core.NewEngine(ctx, config)
	if err != nil {
		slog.Error("failed to create engine", "error", err.Error())
		os.Exit(1)
	}

	slog.SetLogLoggerLevel(config.LogLevel)

	if *isTest {
		// engine.FetchDelegateDelegationState(ctx, tezos.MustParseAddress("tz1gXWW1q8NcXtVy2oVVcc2s4XKNzv9CryWd"), 746, &core.DebugFetchOptions)
		// engine.FetchDelegateDelegationState(ctx, tezos.MustParseAddress("tz1bZ8vsMAXmaWEV7FRnyhcuUs2fYMaQ6Hkk"), 746, &core.DebugFetchOptions)
		// engine.FetchDelegateDelegationState(ctx, tezos.MustParseAddress("tz1ZY5ug2KcAiaVfxhDKtKLx8U5zEgsxgdjV"), 745, &core.DebugFetchOptions)
		engine.FetchCycleDelegationStates(ctx, int64(744), &core.ForceFetchOptions)
		engine.FetchCycleDelegationStates(ctx, int64(745), &core.ForceFetchOptions)
		engine.FetchCycleDelegationStates(ctx, int64(746), &core.ForceFetchOptions)
		return
	}

	publicApiApp := api.CreatePublicApi(config, engine)
	privateApiApp := api.CreatePrivateApi(config, engine)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
	publicApiApp.Shutdown()
	if privateApiApp != nil {
		privateApiApp.Shutdown()
	}
	cancel()
}
