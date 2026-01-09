package backup

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewBackupScheduler(t *testing.T) {
	config := BackupConfig{
		Schedule:      "0 0 * * *",
		RetentionDays: 30,
		BackupPath:    "./test_backups",
		Host:          "localhost",
		Port:          "5432",
		Username:      "postgres",
		Database:      "test_db",
		IncludeSchema: true,
		IncludeData:   true,
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewBackupScheduler(config, logger)

	if scheduler == nil {
		t.Fatal("expected scheduler to be created")
	}

	if scheduler.config.RetentionDays != 30 {
		t.Errorf("expected retention days to be 30, got %d", scheduler.config.RetentionDays)
	}
}

func TestValidateBackup(t *testing.T) {
	config := BackupConfig{
		BackupPath: "./test_backups",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewBackupScheduler(config, logger)

	// Create test directory
	os.MkdirAll(config.BackupPath, 0755)
	defer os.RemoveAll(config.BackupPath)

	tests := []struct {
		name        string
		setupFile   func(string) error
		expectError bool
	}{
		{
			name: "valid backup file",
			setupFile: func(path string) error {
				return os.WriteFile(path, []byte("-- PostgreSQL database dump\nCREATE TABLE test;"), 0644)
			},
			expectError: false,
		},
		{
			name: "empty backup file",
			setupFile: func(path string) error {
				return os.WriteFile(path, []byte(""), 0644)
			},
			expectError: true,
		},
		{
			name: "non-existent file",
			setupFile: func(path string) error {
				return nil // Don't create file
			},
			expectError: true,
		},
		{
			name: "file too small",
			setupFile: func(path string) error {
				return os.WriteFile(path, []byte("test"), 0644)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(config.BackupPath, "test_backup.sql")

			if err := tt.setupFile(testFile); err != nil {
				t.Fatalf("failed to setup test file: %v", err)
			}

			err := scheduler.ValidateBackup(testFile)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			// Cleanup
			os.Remove(testFile)
		})
	}
}

func TestCleanupOldBackups(t *testing.T) {
	config := BackupConfig{
		BackupPath:    "./test_backups",
		RetentionDays: 7,
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
	os.WriteFile(recentFile, []byte("test"), 0644)
	os.Chtimes(recentFile, now, now)

	// Old file (should be deleted)
	oldFile := filepath.Join(config.BackupPath, "sprint_management_20240101_120000.sql")
	os.WriteFile(oldFile, []byte("test"), 0644)
	oldTime := now.AddDate(0, 0, -10) // 10 days old
	os.Chtimes(oldFile, oldTime, oldTime)

	// Non-backup file (should be ignored)
	otherFile := filepath.Join(config.BackupPath, "other_file.txt")
	os.WriteFile(otherFile, []byte("test"), 0644)

	// Run cleanup
	err := scheduler.CleanupOldBackups()
	if err != nil {
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

	// Verify other file still exists
	if _, err := os.Stat(otherFile); os.IsNotExist(err) {
		t.Error("non-backup file was deleted")
	}
}

func TestIsBackupFile(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"sprint_management_20240115_120000.sql", true},
		{"sprint_management_20231201_093045.sql", true},
		{"other_file.sql", false},
		{"sprint_management.sql", false},
		{"backup.txt", false},
		{"sprint_management_backup.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := isBackupFile(tt.filename)
			if result != tt.expected {
				t.Errorf("isBackupFile(%s) = %v, expected %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestSchedulerStartStop(t *testing.T) {
	config := BackupConfig{
		BackupPath:    "./test_backups",
		RetentionDays: 7,
		Host:          "localhost",
		Port:          "5432",
		Username:      "postgres",
		Database:      "test_db",
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewBackupScheduler(config, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start scheduler with short interval for testing
	scheduler.Start(ctx, 100*time.Millisecond)

	// Let it run briefly
	time.Sleep(150 * time.Millisecond)

	// Stop scheduler
	scheduler.Stop()

	// Verify it stopped (no panic or hanging)
	time.Sleep(50 * time.Millisecond)
}

func TestCleanupWithNoRetention(t *testing.T) {
	config := BackupConfig{
		BackupPath:    "./test_backups",
		RetentionDays: 0, // No retention
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	scheduler := NewBackupScheduler(config, logger)

	// Create test directory
	os.MkdirAll(config.BackupPath, 0755)
	defer os.RemoveAll(config.BackupPath)

	// Create old file
	oldFile := filepath.Join(config.BackupPath, "sprint_management_20240101_120000.sql")
	os.WriteFile(oldFile, []byte("test"), 0644)

	// Run cleanup
	err := scheduler.CleanupOldBackups()
	if err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	// Verify file still exists (no cleanup when retention is 0)
	if _, err := os.Stat(oldFile); os.IsNotExist(err) {
		t.Error("file was deleted when retention is 0")
	}
}
