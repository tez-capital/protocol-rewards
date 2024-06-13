package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/tez-capital/ogun/configuration"
	"github.com/trilitech/tzgo/tezos"
	"gorm.io/gorm"
)

type BalanceUpdate struct {
	Kind   string `json:"kind"`
	Change string `json:"change"`
	// Other fields can be added if needed
}

type minDelegatedInCurrentCycle struct {
	Amount string `json:"amount"`
	Level  struct {
		Level              int  `json:"level"`
		LevelPosition      int  `json:"level_position"`
		Cycle              int  `json:"cycle"`
		CyclePosition      int  `json:"cycle_position"`
		ExpectedCommitment bool `json:"expected_commitment"`
	} `json:"level"`
}

func FetchDelegateData(delegateAddress string, db *gorm.DB, config *configuration.Runtime) error {
	tezosSubsystemConfiguration, err := config.GetTezosConfiguration()
	if err != nil {
		return err
	}

	if len(tezosSubsystemConfiguration.Providers) == 0 {
		return errors.New("no valid rpc available")
	}

	rpcUrl := "https://eu.rpc.tez.capital/"
	collector, err := InitDefaultRpcAndTzktColletor(rpcUrl)
	if err != nil {
		slog.Error("failed to initialize collector", "error", err.Error())
		return err
	}

	ctx := context.Background()

	delegate, err := collector.GetDelegateStateFromCycle(ctx, 1, tezos.MustParseAddress(delegateAddress))
	if err != nil {
		slog.Error("failed to fetch delegate state", "error", err.Error())
		return err
	}

	// d, err := json.MarshalIndent(delegate, "", "\t")
	// fmt.Println(string(d))
	// os.Exit(0)

	slog.Info("getting delegation state")
	state, err := collector.GetDelegationState(ctx, delegate)
	if err != nil {
		slog.Error("failed to fetch delegation state", "error", err.Error())
		return err
	}
	result, err := json.MarshalIndent(state, "", "\t")
	fmt.Println(string(result))
	return nil

}
