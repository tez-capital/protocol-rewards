package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/tez-capital/ogun/core"
	"github.com/tez-capital/ogun/store"
)

type databaseConfiguration struct {
	host     string
	port     string
	user     string
	password string
	database string
}

func main() {
	config := databaseConfiguration{
		host:     "127.0.0.1",
		port:     "5432",
		user:     "tezwatch1",
		password: "tezwatch1",
		database: "tezwatch1",
	}

	app := fiber.New()

	store.ConnectDatabase(config.host, config.user, config.password, config.database, config.port)

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

		if err := core.FetchDelegateData(address, store.DB); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.SendStatus(fiber.StatusOK)
	})

	app.Listen(":3000")
}
