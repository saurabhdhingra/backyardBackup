# Backyard Backup

Backyard Backup is a flexible and extensible database backup solution that supports multiple database engines and storage providers.

## Features

- Multiple database engine support:
  - SQLite
  - MySQL (planned)
  - PostgreSQL (planned)
  - MongoDB (planned)
- Various backup types:
  - Full backups
  - Incremental backups (planned)
  - Differential backups (planned)
- Multiple storage providers:
  - Local filesystem
  - AWS S3 (planned)
  - Google Cloud Storage (planned)
  - Azure Blob Storage (planned)
- Backup scheduling (planned)
- Backup compression
- Selective table backups
- Backup listing and management
- Configurable logging
- Notifications (planned)

## Installation

### From Source

```bash
git clone https://github.com/yourusername/backyardBackup.git
cd backyardBackup
go build -o dbbackup ./cmd/dbbackup
```

## Quick Start

1. Create a configuration file based on the example:

```bash
cp config.json.example config.json
```

2. Edit the configuration file to suit your needs.

3. Run a backup:

```bash
./dbbackup -backup -db myLocalSQLite -storage localBackups
```

4. List backups:

```bash
./dbbackup -list -db myLocalSQLite -storage localBackups
```

## Configuration

The configuration file (`config.json`) should be placed in one of these locations:

- The current directory
- `$HOME/.backyardBackup/config.json`
- `/etc/backyardBackup/config.json`

Or specify a custom location with the `-config` flag.

See `config.json.example` for a complete example configuration.

## Usage

### Command Line Options

```
Usage of dbbackup:
  -backup
        Perform a backup
  -compress
        Compress backup (default true)
  -config string
        Path to configuration file
  -db string
        Database name from configuration
  -exclude string
        Tables to exclude (comma-separated)
  -id string
        Backup ID for restore
  -include string
        Tables to include (comma-separated)
  -list
        List available backups
  -output string
        Output directory for restore
  -restore
        Perform a restore
  -storage string
        Storage name from configuration
  -type string
        Backup type (full, incremental, differential) (default "full")
```

### Examples

#### Perform a full backup:

```bash
./dbbackup -backup -db myLocalSQLite -storage localBackups
```

#### Perform a compressed backup of specific tables:

```bash
./dbbackup -backup -db myLocalSQLite -storage localBackups -compress -include "users,orders"
```

#### List available backups:

```bash
./dbbackup -list -db myLocalSQLite -storage localBackups
```

## Project Structure

```
cmd/
  ├── dbbackup/
  │   └── main.go          // Main entry point
internal/
  ├── backup/              // Backup operations
  │   ├── backup.go        // Core backup interface
  │   ├── full.go          // Full backup implementation
  │   ├── incremental.go   // Incremental backup implementation (planned)
  │   └── differential.go  // Differential backup implementation (planned)
  ├── restore/             // Restore operations
  │   ├── restore.go       // Core restore interface
  │   └── selective.go     // Selective restore implementation (planned)
  ├── database/            // Database adapters
  │   ├── connector.go     // Database connector interface
  │   ├── mysql.go         // MySQL implementation (planned)
  │   ├── postgres.go      // PostgreSQL implementation (planned)
  │   ├── mongodb.go       // MongoDB implementation (planned)
  │   └── sqlite.go        // SQLite implementation
  ├── storage/             // Storage providers
  │   ├── provider.go      // Storage provider interface
  │   ├── local.go         // Local storage implementation
  │   ├── s3.go            // AWS S3 implementation (planned)
  │   ├── gcs.go           // Google Cloud Storage implementation (planned)
  │   └── azure.go         // Azure Blob Storage implementation (planned)
  ├── scheduler/           // Backup scheduling (planned)
  │   └── scheduler.go     // Scheduler implementation (planned)
  ├── compression/         // Compression utilities (planned)
  │   └── compression.go   // Compression implementation (planned)
  ├── logging/             // Logging system
  │   └── logger.go        // Logger implementation
  └── notification/        // Notification system (planned)
      └── slack.go         // Slack notifications (planned)
config/
  └── config.go            // Configuration management
pkg/
  └── utils/               // Utility functions (planned)
      └── utils.go         // Utility functions (planned)
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
