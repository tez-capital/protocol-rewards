package api

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/tez-capital/ogun/configuration"
	"github.com/tez-capital/ogun/core"
	"github.com/tez-capital/ogun/store"
)

func FetchCycle(app *fiber.App, config *configuration.Runtime) {
	app.Get("/fetch/:cycle", func(c *fiber.Ctx) error {
		if !config.AllowManualCycleFetching {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "manual cycle fetching is not allowed",
			})
		}
		str := c.Params("cycle")

		cycle, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		delegatesStates, err := core.FetchAllDelegatesStatesFromCycle(int64(cycle), config)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		if err = store.StoreDelegatesStates(delegatesStates); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		return c.SendStatus(fiber.StatusOK)
	})
}

// app.Get("/delegate/:address", func(c *fiber.Ctx) error {
// 	address := c.Params("address")

// 	var delegate store.Delegate
// 	if err := store.DB.Where("address = ?", address).First(&delegate).Error; err != nil {
// 		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
// 			"error": "Delegate with address " + address + " not found",
// 		})
// 	}

// 	return c.JSON(delegate)
// })
