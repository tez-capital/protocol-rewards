package core

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"

// 	"log/slog"

// 	"github.com/trilitech/tzgo/tezos"
// 	"gorm.io/gorm"
// )

// func FetchDelegateData2(delegateAddress string, db *gorm.DB) error {
// 	rpcUrl := "https://rpc.tzkt.io/mainnet/"
// 	collector, err := InitDefaultRpcAndTzktColletor(rpcUrl)
// 	if err != nil {
// 		slog.Error("failed to initialize collector", "error", err.Error())
// 		return err
// 	}

// 	ctx := context.Background()

// 	delegate, err := collector.GetDelegateStateFromCycle(ctx, 1, tezos.MustParseAddress(delegateAddress))
// 	if err != nil {
// 		slog.Error("failed to fetch delegate state", "error", err.Error())
// 		return err
// 	}

// 	slog.Info("getting delegation state")
// 	state, err := collector.GetDelegationState(ctx, delegate)
// 	if err != nil {
// 		slog.Error("failed to fetch delegation state", "error", err.Error())
// 		return err
// 	}
// 	result, err := json.MarshalIndent(state, "", "\t")
// 	fmt.Println(result)
// 	// client, err := rpc.NewClient("https://rpc.tzkt.io/mainnet", nil)
// 	// if err != nil {
// 	// 	return err
// 	// }

// 	// ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

// 	// // Fetch the block details using tzgo
// 	// headBlock, err := client.GetBlock(ctx, rpc.Head)
// 	// if err != nil {
// 	// 	return fmt.Errorf("failed to fetch previous block: %w", err)
// 	// }

// 	// fmt.Println(headBlock)

// 	// // Insert or update data in the database
// 	// // db.Save(&models.Delegate{...})

// 	// cancel()
// 	return nil

// }
