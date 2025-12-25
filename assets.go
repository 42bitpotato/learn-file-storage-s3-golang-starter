package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

//func getAssetPath(videoID uuid.UUID, mediaType string) string {
//	ext := mediaTypeToExt(mediaType)
//	return fmt.Sprintf("%s%s", videoID, ext)
//}

func getAssetPath(mediaType string) string {
	byteSlice := make([]byte, 32)
	rand.Read(byteSlice)
	fileName := base64.RawURLEncoding.EncodeToString(byteSlice)
	ext := mediaTypeToExt(mediaType)
	return fmt.Sprintf("%s%s", fileName, ext)
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

func getVideoAspectRatio(filePath string) (string, error) {
	ffprobe := exec.Command("ffprobe", "-v, error, -print_format, json, -show_streams %s", filePath)
	stdout := &bytes.Buffer{}
	ffprobe.Stdout = stdout
	err := ffprobe.Run()
	if err != nil {
		return "", err
	}
	var VideoFormat struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	if err = json.Unmarshal(stdout.Bytes(), &VideoFormat); err != nil {
		return "", err
	}

	return aspectLabel(VideoFormat.Width, VideoFormat.Height), nil
}

func aspectLabel(width, height int) string {
	if width*9/height == 16 {
		return "16:9"
	}
	if height*9/width == 16 {
		return "9:16"
	}
	return "other"
}
