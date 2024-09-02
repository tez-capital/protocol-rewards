package api

import (
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/tez-capital/protocol-rewards/configuration"
	"github.com/tez-capital/protocol-rewards/constants"
	"github.com/tez-capital/protocol-rewards/core"
	"github.com/tez-capital/protocol-rewards/store"
	"github.com/trilitech/tzgo/tezos"
)

func registerGetDelegationState(app *fiber.App, engine *core.Engine) {
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

		state, err := engine.GetDelegationState(c.Context(), address, cycle)
		if err != nil {
			if errors.Is(err, constants.ErrNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Delegation state not found",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(state)
	})
}

func registerIsDelegationStateAvailable(app *fiber.App, engine *core.Engine) {
	app.Get("/delegate/:cycle/:address/available", func(c *fiber.Ctx) error {
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

		available, err := engine.IsDelegationStateAvailable(c.Context(), address, cycle)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(available)
	})
}

func registerStatistics(app *fiber.App, engine *core.Engine) {
	app.Get("/statistics/:cycle", func(c *fiber.Ctx) error {
		cycle, err := strconv.ParseInt(c.Params("cycle"), 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		available, err := engine.Statisticts(c.Context(), cycle)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.JSON(available)
	})
}

func registerRewardsSplitMirror(app *fiber.App, engine *core.Engine) {
	app.Get("/v1/rewards/split/:address/:cycle", func(c *fiber.Ctx) error {
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

		state, err := engine.GetDelegationState(c.Context(), address, cycle)
		if err != nil {
			if errors.Is(err, constants.ErrNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Delegation state not found",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		switch state.Status {
		case store.DelegationStateStatusMinimumNotAvailable:
			return c.Status(fiber.StatusNoContent).JSON(fiber.Map{
				"error": "relevant minimum does not exists",
			})
		}

		return c.JSON(state.ToTzktState())
	})
}

func CreatePublicApi(config *configuration.Runtime, engine *core.Engine) *fiber.App {
	app := fiber.New()

	app.Use(limiter.New(limiter.Config{
		Max:        10,
		Expiration: 30 * time.Second,
	}))

	registerGetDelegationState(app, engine)
	registerIsDelegationStateAvailable(app, engine)
	registerRewardsSplitMirror(app, engine)
	registerStatistics(app, engine)

	go func() {
		err := app.Listen(config.Listen)
		if err != nil {
			slog.Error("failed to start public api", "error", err.Error())
		}
		// FIXME This is suboptimal, ideally we should have a way to wait for the server to start
	}()
	return app
}
