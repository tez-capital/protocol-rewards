package main

import (
	"flag"

	"github.com/gofiber/fiber/v2"
	"github.com/tez-capital/ogun/configuration"
	"github.com/tez-capital/ogun/core"
	"github.com/tez-capital/ogun/store"
)

func main() {

	configPath := flag.String("config", "config.hjson", "path to the configuration file")

	flag.Parse()

	config, err := configuration.LoadConfiguration(*configPath)
	if err != nil {
		panic(err)
	}

	app := fiber.New()

	store.ConnectDatabase(
		config.Database.Host,
		config.Database.User,
		config.Database.Password,
		config.Database.Database,
		config.Database.Port,
	)

	app.Get("/delegate/:address", func(c *fiber.Ctx) error {
		address := c.Params("address")

		var delegate store.Delegate
		if err := store.DB.Where("address = ?", address).First(&delegate).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Delegate with address " + address + " not found",
			})
		}

		return c.JSON(delegate)
	})

	app.Get("/fetch/:address", func(c *fiber.Ctx) error {
		address := c.Params("address")

		if err := core.FetchDelegateData(address, store.DB, config); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.SendStatus(fiber.StatusOK)
	})

	app.Listen(":3000")
}
