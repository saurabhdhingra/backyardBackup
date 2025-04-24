package compression

import (
	"compress/gzip"
	"fmt"
	"io"
)

// CompressionType represents the type of compression
type CompressionType string

const (
	// Gzip compression
	Gzip CompressionType = "gzip"
	// None - no compression
	None CompressionType = "none"
)

// Compressor provides compression functionality
type Compressor struct {
	Type CompressionType
}

// NewCompressor creates a new compressor with the specified type
func NewCompressor(compType CompressionType) *Compressor {
	return &Compressor{
		Type: compType,
	}
}

// Compress compresses data from a reader to a writer
func (c *Compressor) Compress(r io.Reader, w io.Writer) error {
	switch c.Type {
	case Gzip:
		return compressGzip(r, w)
	case None:
		_, err := io.Copy(w, r)
		return err
	default:
		return fmt.Errorf("unsupported compression type: %s", c.Type)
	}
}

// Decompress decompresses data from a reader to a writer
func (c *Compressor) Decompress(r io.Reader, w io.Writer) error {
	switch c.Type {
	case Gzip:
		return decompressGzip(r, w)
	case None:
		_, err := io.Copy(w, r)
		return err
	default:
		return fmt.Errorf("unsupported compression type: %s", c.Type)
	}
}

// IsCompressed checks if a file is compressed
func IsCompressed(filename string) (bool, CompressionType) {
	// Simple extension-based check
	if len(filename) > 3 && filename[len(filename)-3:] == ".gz" {
		return true, Gzip
	}
	return false, None
}

// compressGzip compresses data using gzip
func compressGzip(r io.Reader, w io.Writer) error {
	gw := gzip.NewWriter(w)
	defer gw.Close()
	
	_, err := io.Copy(gw, r)
	return err
}

// decompressGzip decompresses gzip data
func decompressGzip(r io.Reader, w io.Writer) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()
	
	_, err = io.Copy(w, gr)
	return err
} 