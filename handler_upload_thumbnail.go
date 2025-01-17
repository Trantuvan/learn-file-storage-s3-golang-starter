package main

import (
	"database/sql"
	"encoding/base64"
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
	// *parse file and put in r.FormFile or r.PostForm
	const maxMemory = 10 << 20
	errMultiForm := r.ParseMultipartForm(maxMemory)
	if errMultiForm != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("handlerUploadThumbnail failed to parse thumbnail videoID %q", videoID), err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("handlerUploadThumbnail no video with videoID %q", videoID), err)
			return
		}
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("handlerUploadThumbnail failed to get videoID %q", videoID), err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, fmt.Sprintf("handlerUploadThumbnail videoID: %q not belong to userID: %q", videoID, userID), nil)
		return
	}

	// *"thumbnail" should match the HTML form input name
	// *get the file key "thumbnail" from r.FormFile
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("handlerUploadThumbnail Unable to parse form file videoID %q", videoID), err)
		return
	}

	defer file.Close()
	files, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("handlerUploadThumbnail Unable to read thumbnail videoID %q", videoID), err)
		return
	}

	encodedFile := base64.StdEncoding.EncodeToString(files)
	thumbnailURL := fmt.Sprintf("data:%s;base64,%s", header.Header.Get("Content-Type"), encodedFile)
	video.ThumbnailURL = &thumbnailURL

	errUpdateVideo := cfg.db.UpdateVideo(video)
	if errUpdateVideo != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("handlerUploadThumbnail Unable to update thumbnail videoID %q", videoID), err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
