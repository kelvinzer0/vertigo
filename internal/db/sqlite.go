package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // SQLite driver
)

// InitDB initializes the SQLite database and creates necessary tables.
func InitDB(dataSourceName string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Ping the database to ensure the connection is established
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create conversations table
	sqlStmt := `
	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		last_updated INTEGER
	);
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		conversation_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id)
	);
	`

	_, err = db.Exec(sqlStmt)
	if err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return db, nil
}

// CloseDB closes the database connection.
func CloseDB(db *sql.DB) {
	if db != nil {
		db.Close()
	}
}
