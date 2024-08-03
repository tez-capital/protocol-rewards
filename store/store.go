package store

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/tez-capital/protocol-rewards/configuration"
	"github.com/tez-capital/protocol-rewards/constants"
	"github.com/trilitech/tzgo/tezos"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Store struct {
	db     *gorm.DB
	config configuration.StorageConfiguration
}

func NewStore(config *configuration.Runtime) (*Store, error) {
	host, port, user, pass, database := config.Database.Unwrap()
	slog.Debug("connecting to database", "host", host, "port", port, "user", user, "database", database)

	gormLogger := logger.Default.LogMode(logger.Silent)

	if config.LogLevel == slog.LevelDebug {
		gormLogger = logger.Default.LogMode(logger.Info)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Shanghai", host, user, pass, database, port),
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&StoredDelegationState{})
	return &Store{
		db:     db,
		config: config.Storage,
	}, nil
}

func (s *Store) GetDelegationState(delegate tezos.Address, cycle int64) (*StoredDelegationState, error) {
	var state StoredDelegationState
	if err := s.db.Model(&StoredDelegationState{}).Where("delegate = ? AND cycle = ?", delegate, cycle).First(&state).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errors.Join(constants.ErrNotFound, err)
		}
		return nil, err
	}
	return &state, nil
}

func (s *Store) StoreDelegationState(state *StoredDelegationState) error {
	// update if exists
	if result := s.db.Model(&StoredDelegationState{}).Where("delegate = ? AND cycle = ?", state.Delegate, state.Cycle).Updates(state); result.RowsAffected > 0 && result.Error == nil {
		return nil
	}

	slog.Debug("storing delegation state", "delegate", state.Delegate.String(), "cycle", state.Cycle)
	if err := s.db.Create(state).Error; err != nil {
		return err
	}
	return nil
}

func (s *Store) PruneDelegationState(cycle int64) error {
	if s.config.Mode != constants.Rolling {
		return nil
	}

	prunedCycle := cycle - int64(s.config.StoredCycles)

	state := &StoredDelegationState{}
	slog.Debug("pruning delegation states smaller than", "cycle", prunedCycle)
	return s.db.Model(&StoredDelegationState{}).Where("cycle < ?", prunedCycle).Delete(state).Error

}

func (s *Store) IsDelegationStateAvailable(delegate tezos.Address, cycle int64) (bool, error) {
	var count int64
	s.db.Model(&StoredDelegationState{}).Where("delegate = ? AND cycle = ?", delegate, cycle).Count(&count)
	return count > 0, nil
}

func (s *Store) GetLastFetchedCycle() (int64, error) {
	var cycle int64

	if err := s.db.Model(&StoredDelegationState{}).Select("cycle").Order("cycle desc").First(&cycle).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	return cycle, nil
}
