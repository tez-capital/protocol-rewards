package store

import (
	"fmt"
	"log/slog"

	slogGorm "github.com/orandin/slog-gorm"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func NewStore(host, port, user, pass, database string) (*Store, error) {
	slog.Debug("connecting to database", "host", host, "port", port, "user", user, "database", database)
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Shanghai", host, user, pass, database, port),
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}), &gorm.Config{
		Logger: slogGorm.New(),
	})
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&StoredDelegationState{})
	return &Store{
		db: db,
	}, nil
}

func (s *Store) GetDelegationState(delegate []byte, cycle int64) (*StoredDelegationState, error) {
	var state StoredDelegationState
	if err := s.db.Where("delegate = ? AND cycle = ?", delegate, cycle).First(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *Store) StoreDelegationState(state *StoredDelegationState) error {
	// update if exists
	if err := s.db.Save(state).Error; err != nil {
		return err
	}
	return nil
}
