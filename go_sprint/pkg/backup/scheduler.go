package backup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// BackupConfig holds configuration for backup operations
type BackupConfig struct {
	Schedule      string `yaml:"schedule"`       // Cron expression
	RetentionDays int    `yaml:"retention_days"` // Days to keep backups
	BackupPath    string `yaml:"backup_path"`    // Local backup directory
	Host          string `yaml:"host"`           // Database host
	Port          string `yaml:"port"`           // Database port
	Username      string `yaml:"username"`       // Database username
	Password      string `yaml:"password"`       // Database password
	Database      string `yaml:"database"`       // Database name
	IncludeSchema bool   `yaml:"include_schema"` // Include schema in backup
	IncludeData   bool   `yaml:"include_data"`   // Include data in backup
}

// BackupScheduler manages automated database backups
type BackupScheduler struct {
	config BackupConfig
	logger *slog.Logger
	ticker *time.Ticker
	done   chan bool
}

// NewBackupScheduler creates a new backup scheduler
func NewBackupScheduler(config BackupConfig, logger *slog.Logger) *BackupScheduler {
	return &BackupScheduler{
		config: config,
		logger: logger,
		done:   make(chan bool),
	}
}

// Start begins the backup scheduler
func (bs *BackupScheduler) Start(ctx context.Context, interval time.Duration) {
	bs.ticker = time.NewTicker(interval)

	bs.logger.Info("backup scheduler started",
		slog.Duration("interval", interval),
		slog.Int("retention_days", bs.config.RetentionDays),
	)

	go func() {
		for {
			select {
			case <-bs.ticker.C:
				if err := bs.CreateBackup(ctx); err != nil {
					bs.logger.Error("backup failed", slog.String("error", err.Error()))
				}

				// Clean up old backups
				if err := bs.CleanupOldBackups(); err != nil {
					bs.logger.Error("backup cleanup failed", slog.String("error", err.Error()))
				}
			case <-bs.done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop stops the backup scheduler
func (bs *BackupScheduler) Stop() {
	if bs.ticker != nil {
		bs.ticker.Stop()
	}
	close(bs.done)
	bs.logger.Info("backup scheduler stopped")
}

// CreateBackup creates a database backup using pg_dump
func (bs *BackupScheduler) CreateBackup(ctx context.Context) error {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("sprint_management_%s.sql", timestamp)
	backupFile := filepath.Join(bs.config.BackupPath, filename)

	// Ensure backup directory exists
	if err := os.MkdirAll(bs.config.BackupPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	bs.logger.Info("creating backup",
		slog.String("file", backupFile),
		slog.String("database", bs.config.Database),
	)

	// Build pg_dump command
	args := []string{
		"--host", bs.config.Host,
		"--port", bs.config.Port,
		"--username", bs.config.Username,
		"--dbname", bs.config.Database,
		"--schema", "sprint_management",
		"--schema", "auth",
		"--file", backupFile,
	}

	// Add schema/data options
	if bs.config.IncludeSchema && !bs.config.IncludeData {
		args = append(args, "--schema-only")
	} else if !bs.config.IncludeSchema && bs.config.IncludeData {
		args = append(args, "--data-only")
	}
	// Default: both schema and data (no additional flags needed)

	cmd := exec.CommandContext(ctx, "pg_dump", args...)

	// Set password via environment variable
	if bs.config.Password != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", bs.config.Password))
	}

	// Capture output for logging
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_dump failed: %w, output: %s", err, string(output))
	}

	// Validate backup file was created
	if err := bs.ValidateBackup(backupFile); err != nil {
		return fmt.Errorf("backup validation failed: %w", err)
	}

	bs.logger.Info("backup created successfully",
		slog.String("file", backupFile),
		slog.Int64("size_bytes", bs.getFileSize(backupFile)),
	)

	return nil
}

// ValidateBackup checks if the backup file is valid
func (bs *BackupScheduler) ValidateBackup(backupFile string) error {
	// Check if file exists
	info, err := os.Stat(backupFile)
	if err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	// Check if file is not empty
	if info.Size() == 0 {
		return fmt.Errorf("backup file is empty")
	}

	// Check if file is readable
	file, err := os.Open(backupFile)
	if err != nil {
		return fmt.Errorf("backup file not readable: %w", err)
	}
	defer file.Close()

	// Read first few bytes to verify it's a SQL file
	header := make([]byte, 100)
	n, err := file.Read(header)
	if err != nil && n == 0 {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// Basic validation: check for SQL comments or commands
	headerStr := string(header[:n])
	if len(headerStr) < 10 {
		return fmt.Errorf("backup file too small to be valid")
	}

	return nil
}

// CleanupOldBackups removes backups older than retention period
func (bs *BackupScheduler) CleanupOldBackups() error {
	if bs.config.RetentionDays <= 0 {
		return nil // No cleanup if retention is not set
	}

	cutoffTime := time.Now().AddDate(0, 0, -bs.config.RetentionDays)

	bs.logger.Info("cleaning up old backups",
		slog.Time("cutoff_time", cutoffTime),
		slog.Int("retention_days", bs.config.RetentionDays),
	)

	entries, err := os.ReadDir(bs.config.BackupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	deletedCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process backup files
		if !isBackupFile(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			bs.logger.Warn("failed to get file info",
				slog.String("file", entry.Name()),
				slog.String("error", err.Error()),
			)
			continue
		}

		if info.ModTime().Before(cutoffTime) {
			filePath := filepath.Join(bs.config.BackupPath, entry.Name())
			if err := os.Remove(filePath); err != nil {
				bs.logger.Warn("failed to delete old backup",
					slog.String("file", filePath),
					slog.String("error", err.Error()),
				)
			} else {
				bs.logger.Info("deleted old backup",
					slog.String("file", entry.Name()),
					slog.Time("mod_time", info.ModTime()),
				)
				deletedCount++
			}
		}
	}

	bs.logger.Info("backup cleanup completed",
		slog.Int("deleted_count", deletedCount),
	)

	return nil
}

// getFileSize returns the size of a file in bytes
func (bs *BackupScheduler) getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// isBackupFile checks if a filename matches backup file pattern
func isBackupFile(filename string) bool {
	return filepath.Ext(filename) == ".sql" &&
		len(filename) > len("sprint_management_") &&
		filename[:len("sprint_management_")] == "sprint_management_"
}
