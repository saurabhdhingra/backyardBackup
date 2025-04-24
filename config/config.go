package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/yourusername/backyardBackup/internal/database"
	"github.com/yourusername/backyardBackup/internal/storage"
)

// LogLevel defines the log level
type LogLevel string

const (
	// Debug log level
	Debug LogLevel = "debug"
	// Info log level
	Info LogLevel = "info"
	// Warning log level
	Warning LogLevel = "warning"
	// Error log level
	Error LogLevel = "error"
)

// DatabaseConfig contains database configuration
type DatabaseConfig struct {
	Type     database.DBType
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
	FilePath string // For SQLite
	Options  map[string]string
}

// StorageConfig contains storage configuration
type StorageConfig struct {
	Type      storage.StorageType
	BasePath  string
	Bucket    string
	Region    string
	Endpoint  string
	AccessKey string
	SecretKey string
	Options   map[string]string
}

// BackupSchedule defines when backups should occur
type BackupSchedule struct {
	FullBackup        string // Cron expression for full backups
	IncrementalBackup string // Cron expression for incremental backups
	DifferentialBackup string // Cron expression for differential backups
	RetentionDays     int    // Number of days to keep backups
	MaxBackups        int    // Maximum number of backups to keep
}

// NotificationConfig contains notification settings
type NotificationConfig struct {
	SlackWebhookURL string
	EmailSMTP       string
	EmailFrom       string
	EmailTo         []string
	OnSuccess       bool
	OnFailure       bool
}

// Config represents the application configuration
type Config struct {
	Databases     map[string]DatabaseConfig
	Storage       map[string]StorageConfig
	Schedules     map[string]BackupSchedule
	Notifications NotificationConfig
	LogLevel      LogLevel
	LogFile       string
	DataDir       string
	Compression   bool
	Concurrency   int
	Timeout       time.Duration
}

// LoadConfig loads configuration from a file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults if needed
	if cfg.LogLevel == "" {
		cfg.LogLevel = Info
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Minute
	}
	if cfg.DataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			cfg.DataDir = filepath.Join(homeDir, ".backyardBackup")
		} else {
			cfg.DataDir = ".backyardBackup"
		}
	}

	return &cfg, nil
}

// SaveConfig saves configuration to a file
func SaveConfig(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Databases: map[string]DatabaseConfig{},
		Storage:   map[string]StorageConfig{},
		Schedules: map[string]BackupSchedule{},
		LogLevel:  Info,
		Concurrency: 1,
		Timeout:   30 * time.Minute,
		Compression: true,
	}
} 