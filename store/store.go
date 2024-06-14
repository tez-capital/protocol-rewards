package store

import (
	"fmt"
	"log"
	"log/slog"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDatabase(host, user, pass, database, port string) {
	slog.Debug("connecting to database", "host", host, "port", port, "user", user, "database", database)
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN:                  fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Shanghai", host, user, pass, database, port),
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal("Failed to connect to database", err)
	}

	// Auto migrate the DelegationState struct
	if err := db.AutoMigrate(&DelegationState{}); err != nil {
		fmt.Println("Error migrating database:", err)
		return
	}
	DB = db
}

func StoreDelegatesStates(records []*DelegationState) error {
	if err := DB.Create(&records).Error; err != nil {
		return fmt.Errorf("error saving delegates to database: %v", err)
	}

	return nil
}
