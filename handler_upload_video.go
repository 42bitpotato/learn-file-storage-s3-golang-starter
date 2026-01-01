package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading video ", videoID, "by user", userID)

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't find video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to update this video", nil)
		return
	}

	file, fileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	contentHeader := fileHeader.Header.Get("Content-Type")
	if contentHeader == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type for thumbnail", nil)
		return
	}
	mediaType, _, err := mime.ParseMediaType(contentHeader)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid media parameter in request header", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Video needs to be of type mp4", err)
		return
	}

	tempFile, err := os.CreateTemp("", "tubely_upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create temporary video file", err)
		return
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())

	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error copying video to temp file", err)
		return
	}
	tempFile.Seek(0, io.SeekStart)

	aspectRatio, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting video aspect ratio", err)
		return
	}

	var keyPrefix string
	switch aspectRatio {
	case "16:9":
		keyPrefix = "landscape"
	case "9:16":
		keyPrefix = "portrait"
	default:
		keyPrefix = "other"
	}

	processedFilePath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error processing video to with ffmpeg", err)
		return
	}
	processedFile, err := os.Open(processedFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error opening processed video file", err)
		return
	}
	defer processedFile.Close()
	defer os.Remove(processedFile.Name())

	assetPath := getAssetPath(mediaType)
	key := path.Join(keyPrefix, assetPath)
	s3Params := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &key,
		Body:        processedFile,
		ContentType: &mediaType,
	}
	_, err = cfg.s3Client.PutObject(r.Context(), &s3Params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error uploading video to S3 server", err)
		return
	}

	dbVideoURL := fmt.Sprintf("%s,%s", cfg.s3Bucket, key)
	video.VideoURL = &dbVideoURL
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	preSignedVideo, err := cfg.dbVideoToSignedVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error presigning video url", err)
		return
	}

	respondWithJSON(w, http.StatusOK, preSignedVideo)
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := fmt.Sprintf("%s.processing", filePath)
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg error: %v", err)
	}
	return outputFilePath, nil
}
