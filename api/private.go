package api

import (
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/tez-capital/ogun/configuration"
	"github.com/tez-capital/ogun/core"
	"github.com/trilitech/tzgo/tezos"
)

func registerFetchCycle(app *fiber.App, engine *core.Engine) {
	app.Get("/cycle/:cycle", func(c *fiber.Ctx) error {
		cycle, err := strconv.ParseInt(c.Params("cycle"), 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		go engine.FetchCycleDelegationStates(c.Context(), cycle, c.Params("force") == "true")
		return c.JSON(fiber.Map{
			"cycle": cycle,
		})
	})
}

func registerFetchDelegate(app *fiber.App, engine *core.Engine) {
	app.Get("/delegate/:cycle/:address", func(c *fiber.Ctx) error {
		cycle, err := strconv.ParseInt(c.Params("cycle"), 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		address, err := tezos.ParseAddress(c.Params("address"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		go engine.FetchDelegateDelegationState(c.Context(), address, cycle, c.Params("force") == "true")
		return c.JSON(fiber.Map{
			"cycle":   cycle,
			"address": address.String(),
		})
	})
}

func CreatePrivateApi(config *configuration.Runtime, engine *core.Engine) *fiber.App {
	if config.PrivateListen == "" {
		return nil
	}
	app := fiber.New()
	registerFetchCycle(app, engine)
	registerFetchDelegate(app, engine)

	go func() {
		err := app.Listen(config.PrivateListen)
		if err != nil {
			slog.Error("failed to start public api", "error", err.Error())
		}
		// FIXME This is suboptimal, ideally we should have a way to wait for the server to start
	}()
	return app
}
