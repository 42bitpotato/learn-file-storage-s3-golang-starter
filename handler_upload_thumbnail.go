package main

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, imgHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse formfile", err)
		return
	}
	mType := imgHeader.Header.Get("Content-Type")
	if mType == "" {
		respondWithError(w, http.StatusBadRequest, "Unable to parse media type", err)
	}

	imgData, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error reading image data", err)
		return
	}
	dbMetaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Metadata not found in library", err)
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Error fetching metadata from database", err)
		return
	}
	if dbMetaData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized request", err)
		return
	}
	videoThumbnails[videoID] = thumbnail{
		data:      imgData,
		mediaType: mType,
	}

	// dbMetaData.ThumbnailURL =
	// err = cfg.db.UpdateVideo()
	respondWithJSON(w, http.StatusOK, struct{}{})
}
