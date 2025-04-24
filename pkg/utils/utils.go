package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FormatFileSize formats a file size in bytes to a human-readable format
func FormatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats a duration in a human-readable way
func FormatDuration(d time.Duration) string {
	// Round to seconds
	d = d.Round(time.Second)
	
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// EnsureDirectory creates a directory if it doesn't exist
func EnsureDirectory(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}

// CreateTempFile creates a temporary file with the given prefix and suffix
func CreateTempFile(prefix, suffix string) (*os.File, error) {
	tmpDir := os.TempDir()
	if err := EnsureDirectory(tmpDir); err != nil {
		return nil, err
	}
	
	return os.CreateTemp(tmpDir, prefix+"*"+suffix)
}

// IsPathWritable checks if a path is writable
func IsPathWritable(path string) bool {
	// Check if the directory exists
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	
	// Check if it's a directory
	if !info.IsDir() {
		return false
	}
	
	// Try to create a temporary file
	tmpFile := filepath.Join(path, ".write_test")
	file, err := os.Create(tmpFile)
	if err != nil {
		return false
	}
	
	// Clean up
	file.Close()
	os.Remove(tmpFile)
	
	return true
}

// ParseTables parses a comma-separated list of tables
func ParseTables(tableStr string) []string {
	if tableStr == "" {
		return nil
	}
	
	tables := strings.Split(tableStr, ",")
	for i, table := range tables {
		tables[i] = strings.TrimSpace(table)
	}
	
	return tables
}

// ContainsString checks if a string slice contains a given string
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
} 