package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/tez-capital/ogun/api"
	"github.com/tez-capital/ogun/configuration"
	"github.com/tez-capital/ogun/core"
	"github.com/trilitech/tzgo/tezos"
)

func run_test(ctx context.Context, testFlag string, config *configuration.Runtime) {
	engine, err := core.NewEngine(ctx, config, core.TestEngineOptions)
	if err != nil {
		slog.Error("failed to create engine", "error", err.Error())
		os.Exit(1)
	}

	params := strings.Split(testFlag, ":")
	if len(params) > 1 {
		address := params[0]
		cycle, err := strconv.ParseInt(params[1], 10, 64)
		if err != nil {
			slog.Error("cycle is not int", "error", err)
			showTestExample()
			return
		}

		engine.FetchDelegateDelegationState(ctx, tezos.MustParseAddress(address), cycle, &core.DebugFetchOptions)
		return
	}

	cycle, err := strconv.ParseInt(params[0], 10, 64)
	if err != nil {
		slog.Error("cycle is not int", "error", err)
		showTestExample()
		return
	}

	engine.FetchCycleDelegationStates(ctx, cycle, &core.ForceFetchOptions)
	return
}

func main() {
	configPath := flag.String("config", "config.hjson", "path to the configuration file")
	logLevel := flag.String("log", "", "set the desired log level")
	isTest := flag.String("test", "", "run tests")

	ctx, cancel := context.WithCancel(context.Background())

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Printf("%s -config <path/to/config.json>\n", os.Args[0])
		fmt.Printf("%s -log <logLevel> (debug, info, warn, error)\n", os.Args[0])
		fmt.Printf("%s -test <address>:<cycle> or <cycle>\n", os.Args[0])
	}

	flag.Parse()

	config, err := configuration.LoadConfiguration(*configPath)
	if err != nil {
		panic(err)
	}

	if *logLevel != "" {
		config.LogLevel = configuration.GetLogLevel(*logLevel)
	}

	slog.SetLogLoggerLevel(config.LogLevel)

	switch {
	case *isTest != "":
		run_test(ctx, *isTest, config)
		return
	}

	engine, err := core.NewEngine(ctx, config, core.DefaultEngineOptions)
	if err != nil {
		slog.Error("failed to create engine", "error", err.Error())
		os.Exit(1)
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

func showTestExample() {
	slog.Error("check test parameters again")
	fmt.Println("\nExamples:")
	fmt.Printf("%s -test <address>:<cycle>\n", os.Args[0])
	fmt.Printf("%s -test <cycle>\n", os.Args[0])
}
