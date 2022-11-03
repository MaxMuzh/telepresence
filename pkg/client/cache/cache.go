package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/telepresenceio/telepresence/v2/pkg/filelocation"
)

// ensureCacheDir returns the full path to the directory "telepresence", parented by the directory
// returned by UserCacheDir(). The directory is created if it does not exist.
func ensureCacheDir(ctx context.Context) (string, error) {
	cacheDir, err := filelocation.AppUserCacheDir(ctx)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return "", err
	}
	return cacheDir, nil
}

func SaveToUserCache(ctx context.Context, object any, file string) error {
	jsonContent, err := json.Marshal(object)
	if err != nil {
		return err
	}
	dir, err := ensureCacheDir(ctx)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, file), jsonContent, 0o600)
}

func LoadFromUserCache(ctx context.Context, dest any, file string) error {
	dir, err := filelocation.AppUserCacheDir(ctx)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, file)
	jsonContent, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(jsonContent, &dest); err != nil {
		return fmt.Errorf("failed to parse JSON from file %s: %w", path, err)
	}
	return nil
}

func DeleteFromUserCache(ctx context.Context, file string) error {
	dir, err := filelocation.AppUserCacheDir(ctx)
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, file)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
