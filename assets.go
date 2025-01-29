package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const ErrNotSupportedMediaType = "not supported media typpe"

var (
	sixteenByNine = "16:9"
	nineBySixteen = "9:16"
	other         = "other"
	landscape     = "landscape"
	portrait      = "portrait"

	supportedMediaType map[string]struct{} = map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
		"video/mp4":  {},
	}

	supportedAspectRatios map[string]aspectRatio = map[string]aspectRatio{
		sixteenByNine: {width: 16, height: 9, tolerance: 0.1},
		nineBySixteen: {width: 9, height: 16, tolerance: 0.1},
	}
)

type aspectRatio struct {
	width     float64
	height    float64
	tolerance float64
}

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
	fileName := base64.RawURLEncoding.EncodeToString(randBytes)
	ext := mediaTypeToExt(mediaType)
	return fmt.Sprintf("%s%s", fileName, ext), nil
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}

func (cfg apiConfig) getS3AssetURL(key string) string {
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
}

func getVideoAspectRatio(filePath string) (string, error) {
	type videoInfo struct {
		Stream []struct {
			Index  int     `json:"index"`
			Height float64 `json:"height"`
			Width  float64 `json:"width"`
		} `json:"streams"`
	}

	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var output bytes.Buffer
	cmd.Stdout = &output

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("getVideoAspectRatio: ffprobe %w", err)
	}

	var info videoInfo
	if err := json.Unmarshal(output.Bytes(), &info); err != nil {
		return "", fmt.Errorf("getVideoAspectRatio: failed to unmarshall video info %w", err)
	}

	if len(info.Stream) == 0 {
		return "", errors.New("getVideoAspectRatio: no video streams found")
	}

	videoRatio := info.Stream[0].Width / info.Stream[0].Height
	for k, v := range supportedAspectRatios {
		expectedRatio := v.width / v.height

		if math.Abs(videoRatio-expectedRatio) <= (expectedRatio * v.tolerance) {
			return k, nil
		}
	}
	return other, nil
}

func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}
