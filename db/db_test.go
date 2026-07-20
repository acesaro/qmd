package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func getBusyTimeout(t *testing.T, db *sql.DB) int {
	var busyTimeout int
	err := db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout)
	if err != nil {
		t.Fatalf("failed to query busy_timeout: %v", err)
	}
	return busyTimeout
}

func TestOpenDatabaseBusyTimeoutDefaults(t *testing.T) {
	orig := os.Getenv("QMD_SQLITE_BUSY_TIMEOUT")
	os.Unsetenv("QMD_SQLITE_BUSY_TIMEOUT")
	defer func() {
		if orig != "" {
			os.Setenv("QMD_SQLITE_BUSY_TIMEOUT", orig)
		}
	}()

	db, err := OpenDatabase(":memory:")
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	defer db.Close()

	timeout := getBusyTimeout(t, db)
	if timeout != 120000 {
		t.Errorf("expected busy_timeout 120000, got %d", timeout)
	}
}

func TestOpenDatabaseBusyTimeoutOverride(t *testing.T) {
	orig := os.Getenv("QMD_SQLITE_BUSY_TIMEOUT")
	os.Setenv("QMD_SQLITE_BUSY_TIMEOUT", "750")
	defer func() {
		if orig != "" {
			os.Setenv("QMD_SQLITE_BUSY_TIMEOUT", orig)
		} else {
			os.Unsetenv("QMD_SQLITE_BUSY_TIMEOUT")
		}
	}()

	db, err := OpenDatabase(":memory:")
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	defer db.Close()

	timeout := getBusyTimeout(t, db)
	if timeout != 750 {
		t.Errorf("expected busy_timeout 750, got %d", timeout)
	}
}

func TestOpenDatabaseBusyTimeoutZero(t *testing.T) {
	orig := os.Getenv("QMD_SQLITE_BUSY_TIMEOUT")
	os.Setenv("QMD_SQLITE_BUSY_TIMEOUT", "0")
	defer func() {
		if orig != "" {
			os.Setenv("QMD_SQLITE_BUSY_TIMEOUT", orig)
		} else {
			os.Unsetenv("QMD_SQLITE_BUSY_TIMEOUT")
		}
	}()

	db, err := OpenDatabase(":memory:")
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	defer db.Close()

	timeout := getBusyTimeout(t, db)
	if timeout != 0 {
		t.Errorf("expected busy_timeout 0, got %d", timeout)
	}
}

func TestOpenDatabaseBusyTimeoutFallback(t *testing.T) {
	orig := os.Getenv("QMD_SQLITE_BUSY_TIMEOUT")
	os.Setenv("QMD_SQLITE_BUSY_TIMEOUT", "not-a-number")
	defer func() {
		if orig != "" {
			os.Setenv("QMD_SQLITE_BUSY_TIMEOUT", orig)
		} else {
			os.Unsetenv("QMD_SQLITE_BUSY_TIMEOUT")
		}
	}()

	db, err := OpenDatabase(":memory:")
	if err != nil {
		t.Fatalf("OpenDatabase failed: %v", err)
	}
	defer db.Close()

	timeout := getBusyTimeout(t, db)
	if timeout != 120000 {
		t.Errorf("expected busy_timeout 120000, got %d", timeout)
	}
}

func TestOpenDatabaseContention(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "qmd-busy-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "contention.sqlite")

	// Setup DB schema
	setupDb, err := OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to open setup db: %v", err)
	}
	_, err = setupDb.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)")
	if err != nil {
		setupDb.Close()
		t.Fatalf("failed to create table: %v", err)
	}
	setupDb.Close()

	// Open holder and waiter connections
	holder, err := OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to open holder: %v", err)
	}
	defer holder.Close()

	waiter, err := OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to open waiter: %v", err)
	}
	defer waiter.Close()

	// Set waiter busy timeout to a short duration
	_, err = waiter.Exec("PRAGMA busy_timeout = 250")
	if err != nil {
		t.Fatalf("failed to set busy_timeout on waiter: %v", err)
	}

	// Begin immediate transaction on holder to lock the database
	tx, err := holder.Begin()
	if err != nil {
		t.Fatalf("failed to begin transaction on holder: %v", err)
	}
	_, err = tx.Exec("INSERT INTO t (v) VALUES ('holder')")
	if err != nil {
		t.Fatalf("failed to insert in holder: %v", err)
	}

	start := time.Now()
	_, err = waiter.Exec("BEGIN IMMEDIATE")
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected waiter to fail with SQLITE_BUSY, but it succeeded")
	}

	if elapsed < 200*time.Millisecond {
		t.Errorf("expected busy timeout to wait at least 200ms, waited %v", elapsed)
	}

	// Rollback holder transaction
	tx.Rollback()
}
