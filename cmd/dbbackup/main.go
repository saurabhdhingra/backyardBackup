package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yourusername/backyardBackup/config"
	"github.com/yourusername/backyardBackup/internal/backup"
	"github.com/yourusername/backyardBackup/internal/database"
	"github.com/yourusername/backyardBackup/internal/logging"
	"github.com/yourusername/backyardBackup/internal/storage"
)

var (
	configFile     string
	backupCmd      bool
	restoreCmd     bool
	listBackupsCmd bool
	dbName         string
	storeName      string
	backupType     string
	backupID       string
	compress       bool
	outputDir      string
	includeTables  string
	excludeTables  string
)

func init() {
	// Global flags
	flag.StringVar(&configFile, "config", "", "Path to configuration file")
	
	// Command flags
	flag.BoolVar(&backupCmd, "backup", false, "Perform a backup")
	flag.BoolVar(&restoreCmd, "restore", false, "Perform a restore")
	flag.BoolVar(&listBackupsCmd, "list", false, "List available backups")
	
	// Backup/restore options
	flag.StringVar(&dbName, "db", "", "Database name from configuration")
	flag.StringVar(&storeName, "storage", "", "Storage name from configuration")
	flag.StringVar(&backupType, "type", "full", "Backup type (full, incremental, differential)")
	flag.StringVar(&backupID, "id", "", "Backup ID for restore")
	flag.BoolVar(&compress, "compress", true, "Compress backup")
	flag.StringVar(&outputDir, "output", "", "Output directory for restore")
	flag.StringVar(&includeTables, "include", "", "Tables to include (comma-separated)")
	flag.StringVar(&excludeTables, "exclude", "", "Tables to exclude (comma-separated)")
}

func main() {
	flag.Parse()
	
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}
	
	// Set up logger
	logger, err := logging.NewLogger(cfg.LogLevel, cfg.LogFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()
	
	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Handle signals
	setupSignalHandler(cancel)
	
	// Determine which command to run
	switch {
	case backupCmd:
		err = runBackup(ctx, cfg, logger)
	case restoreCmd:
		err = runRestore(ctx, cfg, logger)
	case listBackupsCmd:
		err = runListBackups(ctx, cfg, logger)
	default:
		flag.Usage()
		os.Exit(1)
	}
	
	if err != nil {
		logger.Error("Command failed: %v", err)
		os.Exit(1)
	}
}

func loadConfig() (*config.Config, error) {
	// If config file not specified, look in default locations
	if configFile == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			// Try home directory
			homePath := filepath.Join(home, ".backyardBackup", "config.json")
			if _, err := os.Stat(homePath); err == nil {
				configFile = homePath
			}
		}
		
		// Try current directory
		if configFile == "" {
			if _, err := os.Stat("config.json"); err == nil {
				configFile = "config.json"
			}
		}
		
		// Try /etc directory
		if configFile == "" {
			if _, err := os.Stat("/etc/backyardBackup/config.json"); err == nil {
				configFile = "/etc/backyardBackup/config.json"
			}
		}
	}
	
	// If config file found, load it
	if configFile != "" {
		return config.LoadConfig(configFile)
	}
	
	// Otherwise, use default configuration
	return config.DefaultConfig(), nil
}

func setupSignalHandler(cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-c
		fmt.Println("\nReceived signal, shutting down...")
		cancel()
		// Give operations a moment to clean up
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}()
}

func runBackup(ctx context.Context, cfg *config.Config, logger *logging.Logger) error {
	// Validate required parameters
	if dbName == "" {
		return fmt.Errorf("database name is required for backup")
	}
	if storeName == "" {
		return fmt.Errorf("storage name is required for backup")
	}
	
	// Check if database configuration exists
	dbConfig, ok := cfg.Databases[dbName]
	if !ok {
		return fmt.Errorf("database %q not found in configuration", dbName)
	}
	
	// Check if storage configuration exists
	storeConfig, ok := cfg.Storage[storeName]
	if !ok {
		return fmt.Errorf("storage %q not found in configuration", storeName)
	}
	
	// Initialize database connector
	var db database.Connector
	switch dbConfig.Type {
	case database.SQLite:
		db = &database.SQLiteConnector{}
	case database.MySQL:
		db = &database.MySQLConnector{}
	case database.PostgreSQL:
		db = &database.PostgreSQLConnector{}
	case database.MongoDB:
		db = &database.MongoDBConnector{}
	default:
		return fmt.Errorf("unsupported database type: %s", dbConfig.Type)
	}
	
	// Connect to database
	dbConnConfig := database.ConnectConfig{
		Type:     dbConfig.Type,
		Host:     dbConfig.Host,
		Port:     dbConfig.Port,
		User:     dbConfig.User,
		Password: dbConfig.Password,
		Database: dbConfig.Database,
		SSLMode:  dbConfig.SSLMode,
		FilePath: dbConfig.FilePath,
		Options:  dbConfig.Options,
	}
	
	logger.Info("Connecting to database %s", dbName)
	if err := db.Connect(ctx, dbConnConfig); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()
	
	// Initialize storage provider
	var store storage.Provider
	switch storeConfig.Type {
	case storage.Local:
		store = &storage.LocalProvider{}
	case storage.S3:
		store = &storage.S3Provider{}
	case storage.GCS:
		// Implement GCS provider
		return fmt.Errorf("GCS storage provider not implemented yet")
	case storage.Azure:
		store = &storage.AzureProvider{}
	default:
		return fmt.Errorf("unsupported storage type: %s", storeConfig.Type)
	}
	
	// Initialize storage
	storeProvConfig := storage.ProviderConfig{
		Type:      storeConfig.Type,
		BasePath:  storeConfig.BasePath,
		Bucket:    storeConfig.Bucket,
		Region:    storeConfig.Region,
		Endpoint:  storeConfig.Endpoint,
		AccessKey: storeConfig.AccessKey,
		SecretKey: storeConfig.SecretKey,
		Options:   storeConfig.Options,
	}
	
	logger.Info("Initializing storage %s", storeName)
	if err := store.Initialize(ctx, storeProvConfig); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	
	// Determine backup type
	var backupTypeEnum backup.BackupType
	switch strings.ToLower(backupType) {
	case "full":
		backupTypeEnum = backup.Full
	case "incremental":
		backupTypeEnum = backup.Incremental
	case "differential":
		backupTypeEnum = backup.Differential
	default:
		return fmt.Errorf("unsupported backup type: %s", backupType)
	}
	
	// Create backup options
	backupOpts := backup.BackupOptions{
		Type:         backupTypeEnum,
		Compress:     compress,
		SourceDB:     dbName,
		DestStorage:  storeName,
		IncludeTables: parseTables(includeTables),
		ExcludeTables: parseTables(excludeTables),
	}
	
	// Create backuper
	var backuper backup.Backuper
	switch backupTypeEnum {
	case backup.Full:
		backuper = backup.NewFullBackup(db, store)
	case backup.Incremental:
		// Implement incremental backup
		return fmt.Errorf("incremental backup not implemented yet")
	case backup.Differential:
		// Implement differential backup
		return fmt.Errorf("differential backup not implemented yet")
	}
	
	// Perform backup
	logger.Info("Starting %s backup of database %s", backupType, dbName)
	result, err := backuper.Backup(ctx, backupOpts)
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	
	// Log result
	logger.Info("Backup completed successfully:")
	logger.Info("  ID:        %s", result.ID)
	logger.Info("  Type:      %s", result.Type)
	logger.Info("  Size:      %d bytes", result.Size)
	logger.Info("  Duration:  %s", result.EndTime.Sub(result.StartTime))
	logger.Info("  Path:      %s", result.StoragePath)
	
	return nil
}

func runRestore(ctx context.Context, cfg *config.Config, logger *logging.Logger) error {
	// Implement restore functionality
	return fmt.Errorf("restore not implemented yet")
}

func runListBackups(ctx context.Context, cfg *config.Config, logger *logging.Logger) error {
	// Validate required parameters
	if dbName == "" {
		return fmt.Errorf("database name is required for listing backups")
	}
	if storeName == "" {
		return fmt.Errorf("storage name is required for listing backups")
	}
	
	// Check if database configuration exists
	dbConfig, ok := cfg.Databases[dbName]
	if !ok {
		return fmt.Errorf("database %q not found in configuration", dbName)
	}
	
	// Check if storage configuration exists
	storeConfig, ok := cfg.Storage[storeName]
	if !ok {
		return fmt.Errorf("storage %q not found in configuration", storeName)
	}
	
	// Initialize database connector
	var db database.Connector
	switch dbConfig.Type {
	case database.SQLite:
		db = &database.SQLiteConnector{}
	case database.MySQL:
		db = &database.MySQLConnector{}
	case database.PostgreSQL:
		db = &database.PostgreSQLConnector{}
	case database.MongoDB:
		db = &database.MongoDBConnector{}
	default:
		return fmt.Errorf("unsupported database type: %s", dbConfig.Type)
	}
	
	// Connect to database
	dbConnConfig := database.ConnectConfig{
		Type:     dbConfig.Type,
		Host:     dbConfig.Host,
		Port:     dbConfig.Port,
		User:     dbConfig.User,
		Password: dbConfig.Password,
		Database: dbConfig.Database,
		SSLMode:  dbConfig.SSLMode,
		FilePath: dbConfig.FilePath,
		Options:  dbConfig.Options,
	}
	
	logger.Info("Connecting to database %s", dbName)
	if err := db.Connect(ctx, dbConnConfig); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()
	
	// Initialize storage provider
	var store storage.Provider
	switch storeConfig.Type {
	case storage.Local:
		store = &storage.LocalProvider{}
	case storage.S3:
		// Implement S3 provider
		return fmt.Errorf("S3 storage provider not implemented yet")
	case storage.GCS:
		// Implement GCS provider
		return fmt.Errorf("GCS storage provider not implemented yet")
	case storage.Azure:
		// Implement Azure provider
		return fmt.Errorf("Azure storage provider not implemented yet")
	default:
		return fmt.Errorf("unsupported storage type: %s", storeConfig.Type)
	}
	
	// Initialize storage
	storeProvConfig := storage.ProviderConfig{
		Type:      storeConfig.Type,
		BasePath:  storeConfig.BasePath,
		Bucket:    storeConfig.Bucket,
		Region:    storeConfig.Region,
		Endpoint:  storeConfig.Endpoint,
		AccessKey: storeConfig.AccessKey,
		SecretKey: storeConfig.SecretKey,
		Options:   storeConfig.Options,
	}
	
	logger.Info("Initializing storage %s", storeName)
	if err := store.Initialize(ctx, storeProvConfig); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	
	// Create backuper
	backuper := backup.NewFullBackup(db, store)
	
	// List backups
	logger.Info("Listing backups for database %s", dbName)
	backups, err := backuper.ListBackups(ctx)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}
	
	// Display results
	if len(backups) == 0 {
		fmt.Println("No backups found.")
		return nil
	}
	
	fmt.Println("Available backups:")
	fmt.Println("ID                                     | Type     | Size       | Date                | Path")
	fmt.Println("--------------------------------------- | -------- | ---------- | ------------------- | ----")
	
	for _, b := range backups {
		fmt.Printf("%-38s | %-8s | %-10d | %-19s | %s\n",
			b.ID,
			b.Type,
			b.Size,
			b.StartTime.Format("2006-01-02 15:04:05"),
			b.StoragePath,
		)
	}
	
	return nil
}

func parseTables(tablesStr string) []string {
	if tablesStr == "" {
		return nil
	}
	
	tables := strings.Split(tablesStr, ",")
	for i, table := range tables {
		tables[i] = strings.TrimSpace(table)
	}
	
	return tables
} 