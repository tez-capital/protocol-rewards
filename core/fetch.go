package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/tez-capital/ogun/configuration"
	"github.com/trilitech/tzgo/rpc"
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

	minBalanceUrl := fmt.Sprintf("%schains/main/blocks/head/context/delegates/%s/min_delegated_in_current_cycle", tezosSubsystemConfiguration.Providers[0], delegateAddress)

	resp, err := http.Get(minBalanceUrl)
	if err != nil {
		return fmt.Errorf("failed to fetch min delegated balance: %w", err)
	}
	defer resp.Body.Close()

	var minDelegatedObject minDelegatedInCurrentCycle
	if err := json.NewDecoder(resp.Body).Decode(&minDelegatedObject); err != nil {
		return fmt.Errorf("failed to decode min delegated balance response: %w", err)
	}

	client, err := rpc.NewClient(tezosSubsystemConfiguration.Providers[0], nil)
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	// Fetch the block details using tzgo
	headBlock, err := client.GetHeadBlock(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch previous block: %w", err)
	}

	fmt.Println(headBlock)

	// Insert or update data in the database
	// db.Save(&models.Delegate{...})

	cancel()
	return nil

}