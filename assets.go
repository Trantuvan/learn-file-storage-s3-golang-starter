package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

var supportedMediaType map[string]struct{} = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
}

const ErrNotSupportedMediaType = "not supported media typpe"

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func (cfg apiConfig) isSupportedMediaType(value string) (bool, error) {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false, err
	}
	if _, ok := supportedMediaType[mediaType]; !ok {
		return false, errors.New(ErrNotSupportedMediaType)
	}
	return true, nil
}

func getAssetPath(mediaType string) (string, error) {
	// *uuid.UUID - [16]byte convert to slice videoID[:]
	randBytes := make([]byte, 32)
	_, err := rand.Read(randBytes)
	if err != nil {
		return "", fmt.Errorf("getAssetPath Failed to read uuid to []byte %w", err)
	}
	fileName := base64.RawStdEncoding.EncodeToString(randBytes)
	ext := mediaTypeToExt(mediaType)
	return fmt.Sprintf("%s%s", fileName, ext), nil
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}

func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}
