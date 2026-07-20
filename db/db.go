package db

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// OpenDatabase opens a SQLite database connection with busy timeout, WAL mode, and foreign keys enabled.
func OpenDatabase(path string) (*sql.DB, error) {
	// Parse QMD_SQLITE_BUSY_TIMEOUT env var
	busyTimeoutMs := 120000
	if rawEnv := os.Getenv("QMD_SQLITE_BUSY_TIMEOUT"); rawEnv != "" {
		if val, err := strconv.Atoi(rawEnv); err == nil && val >= 0 {
			busyTimeoutMs = val
		}
	}

	// Build DSN
	var dsn string
	if path == ":memory:" {
		dsn = fmt.Sprintf("file::memory:?mode=memory&cache=shared&_busy_timeout=%d&_journal_mode=WAL&_foreign_keys=ON", busyTimeoutMs)
	} else {
		// Clean and prepare query parameters
		params := []string{
			fmt.Sprintf("_busy_timeout=%d", busyTimeoutMs),
			"_journal_mode=WAL",
			"_foreign_keys=ON",
		}
		if strings.Contains(path, "?") {
			dsn = path + "&" + strings.Join(params, "&")
		} else {
			dsn = path + "?" + strings.Join(params, "&")
		}
	}

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	return db, nil
}
