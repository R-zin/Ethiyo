package user

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"os"
)
func ConnectDB() (*sql.DB, error) {
	dest := os.Getenv("DATABASE_URL")
	if dest == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}
	
	db, err := sql.Open("postgres", dest)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}
	
	return db, nil
}