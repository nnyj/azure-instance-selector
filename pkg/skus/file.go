package skus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nnyj/azure-instance-selector/pkg/selector"
)

// LoadCached loads SKUs from the cache file for region.
// Returns (skus, nil) if cache exists and is fresher than ttl.
// Returns (nil, nil) if cache is stale (caller should refresh).
// Returns (nil, err) on read/parse errors other than file-not-found.
func LoadCached(cacheDir, region string, ttl time.Duration) ([]selector.VmSku, error) {
	path := cachePath(cacheDir, region)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if time.Since(info.ModTime()) > ttl {
		return nil, nil // stale
	}
	return LoadFile(path)
}

// LoadStale loads SKUs from cache regardless of TTL (fallback with warning).
func LoadStale(cacheDir, region string) ([]selector.VmSku, error) {
	return LoadFile(cachePath(cacheDir, region))
}

// SaveCache writes skus to the cache file for region, creating dirs as needed.
func SaveCache(cacheDir, region string, skus []selector.VmSku) error {
	path := cachePath(cacheDir, region)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(skus)
}

// LoadFile loads SKUs from a normalized JSON file (cache or AZURE_INSTANCE_SELECTOR_SKUS_FILE override).
func LoadFile(path string) ([]selector.VmSku, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("SKU cache not found at %s — run with --refresh or set AZURE_INSTANCE_SELECTOR_SKUS_FILE", path)
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var skus []selector.VmSku
	if err := json.NewDecoder(f).Decode(&skus); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return skus, nil
}

func cachePath(cacheDir, region string) string {
	return filepath.Join(cacheDir, fmt.Sprintf("skus_%s.json", region))
}
