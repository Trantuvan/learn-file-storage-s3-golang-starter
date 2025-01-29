package main

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoID, err := uuid.Parse(r.PathValue("videoID"))
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

	fmt.Println("uploading video", videoID, "by user", userID)
	const maxMemory = 1 << 30
	errMultiForm := r.ParseMultipartForm(maxMemory)
	if errMultiForm != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("handlerUploadVideo failed to parse video %s", videoID), errMultiForm)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("handlerUploadVideo no video with videoID %q", videoID), err)
			return
		}
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("handlerUploadVideo failed to get videoID %q", videoID), err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, fmt.Sprintf("handlerUploadVideo videoID: %q not belong to userID: %q", videoID, userID), nil)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("handlerUploadVideo Unable to parse form file videoID %q", videoID), err)
		return
	}
	defer file.Close()

	if ok, err := cfg.isSupportedMediaType(header.Header.Get("Content-Type")); !ok {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("handlerUploadVideo %s", err), err)
		return
	}

	diskFilePath := cfg.getAssetDiskPath("")
	tempFile, err := os.CreateTemp(diskFilePath, "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("handlerUploadVideo Unable to create file root %s", err), err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, errCopy := io.Copy(tempFile, file)
	if errCopy != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("handlerUploadVideo Unable to copy file to %s", cfg.assetsRoot), err)
		return
	}

	_, errRet := tempFile.Seek(0, io.SeekStart)
	if errRet != nil {
		respondWithError(w, http.StatusInternalServerError, "handlerUploadVideo Unable to reset file to begining for read", err)
		return
	}

	assetsPath, err := getAssetPath(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("handlerUploadVideo Unable to file name %s", err), err)
		return
	}

	aspect, err := getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("handlerUploadVideo failed to get aspect ratio %s", err), err)
		return
	}

	if aspect == sixteenByNine {
		assetsPath = filepath.Join(landscape, assetsPath)
	} else if aspect == nineBySixteen {
		assetsPath = filepath.Join(portrait, assetsPath)
	} else {
		assetsPath = filepath.Join("other", assetsPath)
	}

	contentType := header.Header.Get("Content-Type")
	_, errS3 := cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &assetsPath,
		Body:        tempFile,
		ContentType: &contentType,
	})
	if errS3 != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("handlerUploadVideo Unable to upload file name %s to s3", errS3), errS3)
		return
	}

	videoURL := cfg.getS3AssetURL(assetsPath)
	video.VideoURL = &videoURL

	errUpdateVideo := cfg.db.UpdateVideo(video)
	if errUpdateVideo != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("handlerUploadVideo Unable to update thumbnail videoID %q", videoID), err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
