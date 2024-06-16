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
	"github.com/trilitech/tzgo/tezos"
)

func main() {
	configPath := flag.String("config", "config.hjson", "path to the configuration file")
	isTest := flag.Bool("test", false, "run the test")

	ctx, cancel := context.WithCancel(context.Background())

	flag.Parse()

	config, err := configuration.LoadConfiguration(*configPath)
	if err != nil {
		panic(err)
	}

	engine, err := core.NewEngine(ctx, config)
	if err != nil {
		slog.Error("failed to create engine", "error", err.Error())
		os.Exit(1)
	}

	slog.SetLogLoggerLevel(config.LogLevel)

	if *isTest {
		engine.FetchDelegateDelegationState(ctx, tezos.MustParseAddress("tz1P6WKJu2rcbxKiKRZHKQKmKrpC9TfW1AwM"), 745, true)
		engine.FetchCycleDelegationStates(ctx, int64(746), false)
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
