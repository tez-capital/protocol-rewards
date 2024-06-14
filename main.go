package main

import (
	"flag"

	"github.com/gofiber/fiber/v2"
	"github.com/tez-capital/ogun/api"
	"github.com/tez-capital/ogun/configuration"
	"github.com/tez-capital/ogun/core"
	"github.com/tez-capital/ogun/store"
)

func main() {

	configPath := flag.String("config", "config.hjson", "path to the configuration file")
	isTest := flag.Bool("test", false, "run the test")

	flag.Parse()

	config, err := configuration.LoadConfiguration(*configPath)
	if err != nil {
		panic(err)
	}

	if *isTest {
		// core.FetchDelegateData("tz3LV9aGKHDnAZHCtC9SjNtTrKRu678FqSki", nil, config)
		// core.FetchAllDelegatesFromCycle(int64(745), config)
		core.FetchAllDelegatesStatesFromCycle(int64(745), config)
		return
	}

	app := fiber.New()

	store.ConnectDatabase(
		config.Database.Host,
		config.Database.User,
		config.Database.Password,
		config.Database.Database,
		config.Database.Port,
	)

	api.FetchCycle(app, config)

	app.Listen(config.Listen[0])
}
