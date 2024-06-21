package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/tez-capital/ogun/api"
	"github.com/tez-capital/ogun/configuration"
	"github.com/tez-capital/ogun/constants"
	"github.com/tez-capital/ogun/core"
	"github.com/tez-capital/ogun/test"
	"github.com/trilitech/tzgo/tezos"
)

func run_test(ctx context.Context, testFlag string, config *configuration.Runtime, cacheId *string) {
	options := core.TestEngineOptions
	if cacheId != nil {
		var err error
		options.Transport, err = test.NewTestTransport(http.DefaultTransport, *cacheId, *cacheId+".gob.lz4")
		if err != nil {
			slog.Error("failed to create caching transport", "error", err)
			return
		}
		slog.Info("using caching transport", "cacheId", *cacheId)
	}

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
}

func main() {
	configPath := flag.String("config", "config.hjson", "path to the configuration file")
	logLevel := flag.String("log", "", "set the desired log level")
	isTest := flag.String("test", "", "run tests")
	cacheId := flag.String("cache", "", "cache id")
	versionFlag := flag.Bool("version", false, "print version")

	ctx, cancel := context.WithCancel(context.Background())

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Printf("%s -config <path/to/config.json>\n", os.Args[0])
		fmt.Printf("%s -log <logLevel> (debug, info, warn, error)\n", os.Args[0])
		fmt.Printf("%s -test <address>:<cycle> or <cycle>\n", os.Args[0])
		fmt.Printf("%s -cache test/data/745 (only in combination with -test)\n", os.Args[0])
	}

	flag.Parse()

	if *versionFlag {
		fmt.Println("protocol-rewards version " + constants.VERSION)
		os.Exit(0)
	}

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
		run_test(ctx, *isTest, config, cacheId)
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
