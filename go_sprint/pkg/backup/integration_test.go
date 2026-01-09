package backup

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// TestBackupRestoreIntegration tests the complete backup and restore workflow
// This test requires PostgreSQL to be running
func TestBackupRestoreIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if PostgreSQL is available
	if !isPostgreSQLAvailable() {
		t.Skip("PostgreSQL not available, skipping integration test")
	}

	// Setup test database
	testDB := "test_backup_restore"
	if err := setupTestDatabase(testDB); err != nil {
		t.Fatalf("failed to setup test database: %v", err)
	}
	defer cleanupTestDatabase(testDB)

	// Create test data
	db, err := sql.Open("postgres", fmt.Sprintf("host=localhost port=5432 user=postgres dbname=%s sslmode=disable", testDB))
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	defer db.Close()

	if err := createTestData(db); err != nil {
		t.Fatalf("failed to create test data: %v", err)
	}

	// Create backup
	config := BackupConfig{
		BackupPath:    "./test_integration_backups",
		Host:          "localhost",
		Port:          "5432",
		Username:      "postgres",
		Database:      testDB,
		IncludeSchema: true,
		IncludeData:   true,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewBackupScheduler(config, logger)

	ctx := context.Background()
	if err := scheduler.CreateBackup(ctx); err != nil {
		t.Fatalf("backup creation failed: %v", err)
	}

	// Verify backup file exists
	backupFiles, err := filepath.Glob(filepath.Join(config.BackupPath, "sprint_management_*.sql"))
	if err != nil || len(backupFiles) == 0 {
		t.Fatalf("backup file not found")
	}

	backupFile := backupFiles[0]
	t.Logf("Backup created: %s", backupFile)

	// Validate backup
	if err := scheduler.ValidateBackup(backupFile); err != nil {
		t.Fatalf("backup validation failed: %v", err)
	}

	// Drop test data
	if _, err := db.Exec("DROP TABLE IF EXISTS test_table CASCADE"); err != nil {
		t.Fatalf("failed to drop test table: %v", err)
	}

	// Restore from backup
	if err := restoreBackup(testDB, backupFile); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	// Verify restored data
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&count); err != nil {
		t.Fatalf("failed to query restored data: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 rows after restore, got %d", count)
	}

	// Cleanup
	os.RemoveAll(config.BackupPath)
}

// TestBackupSchedulerIntegration tests the automated backup scheduler
func TestBackupSchedulerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	if !isPostgreSQLAvailable() {
		t.Skip("PostgreSQL not available, skipping integration test")
	}

	testDB := "test_scheduler"
	if err := setupTestDatabase(testDB); err != nil {
		t.Fatalf("failed to setup test database: %v", err)
	}
	defer cleanupTestDatabase(testDB)

	config := BackupConfig{
		BackupPath:    "./test_scheduler_backups",
		RetentionDays: 1,
		Host:          "localhost",
		Port:          "5432",
		Username:      "postgres",
		Database:      testDB,
		IncludeSchema: true,
		IncludeData:   true,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewBackupScheduler(config, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start scheduler with short interval
	scheduler.Start(ctx, 1*time.Second)

	// Wait for at least one backup
	time.Sleep(2 * time.Second)

	// Stop scheduler
	scheduler.Stop()

	// Verify backup was created
	backupFiles, err := filepath.Glob(filepath.Join(config.BackupPath, "sprint_management_*.sql"))
	if err != nil {
		t.Fatalf("failed to list backup files: %v", err)
	}

	if len(backupFiles) == 0 {
		t.Error("no backups were created by scheduler")
	}

	// Cleanup
	os.RemoveAll(config.BackupPath)
}

// TestBackupRetentionIntegration tests the backup retention cleanup
func TestBackupRetentionIntegration(t *testing.T) {
	config := BackupConfig{
		BackupPath:    "./test_retention_backups",
		RetentionDays: 2,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewBackupScheduler(config, logger)

	// Create test directory
	os.MkdirAll(config.BackupPath, 0755)
	defer os.RemoveAll(config.BackupPath)

	// Create test backup files with different ages
	now := time.Now()

	// Recent file (should be kept)
	recentFile := filepath.Join(config.BackupPath, "sprint_management_20240115_120000.sql")
	os.WriteFile(recentFile, []byte("test backup content"), 0644)
	os.Chtimes(recentFile, now, now)

	// Old file (should be deleted)
	oldFile := filepath.Join(config.BackupPath, "sprint_management_20240101_120000.sql")
	os.WriteFile(oldFile, []byte("old backup content"), 0644)
	oldTime := now.AddDate(0, 0, -5) // 5 days old
	os.Chtimes(oldFile, oldTime, oldTime)

	// Run cleanup
	if err := scheduler.CleanupOldBackups(); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	// Verify recent file still exists
	if _, err := os.Stat(recentFile); os.IsNotExist(err) {
		t.Error("recent backup file was deleted")
	}

	// Verify old file was deleted
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old backup file was not deleted")
	}
}

// Helper functions

func isPostgreSQLAvailable() bool {
	cmd := exec.Command("psql", "--version")
	return cmd.Run() == nil
}

func setupTestDatabase(dbName string) error {
	// Connect to postgres database to create test database
	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres dbname=postgres sslmode=disable")
	if err != nil {
		return err
	}
	defer db.Close()

	// Drop if exists
	_, _ = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName))

	// Create test database
	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
	return err
}

func cleanupTestDatabase(dbName string) error {
	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres dbname=postgres sslmode=disable")
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName))
	return err
}

func createTestData(db *sql.DB) error {
	// Create test table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS test_table (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}

	// Insert test data
	_, err = db.Exec(`
		INSERT INTO test_table (name) VALUES 
		('Test 1'),
		('Test 2'),
		('Test 3')
	`)
	return err
}

func restoreBackup(dbName, backupFile string) error {
	cmd := exec.Command("psql",
		"-h", "localhost",
		"-p", "5432",
		"-U", "postgres",
		"-d", dbName,
		"-f", backupFile,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restore failed: %w, output: %s", err, string(output))
	}

	return nil
}
