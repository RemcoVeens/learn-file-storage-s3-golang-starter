package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	const MaxMemory = 10 << 20
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

	err = r.ParseMultipartForm(MaxMemory)
	if err != nil {
		respondWithError(w, http.StatusRequestEntityTooLarge, "Error pacing file", err)
		return
	}
	FileData, FileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't find thumbnail", err)
		return
	}
	defer FileData.Close()
	MediaType := FileHeader.Header.Get("Content-Type")
	bytes, err := io.ReadAll(FileData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't read thumbnail", err)
		return
	}
	b64Img := base64.StdEncoding.EncodeToString(bytes)
	dataURL := fmt.Sprintf("data:%v;base64,%v", MediaType, b64Img)
	Video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Not your video", err)
		return
	}
	Video.ThumbnailURL = &dataURL
	err = cfg.db.UpdateVideo(Video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}
	respondWithJSON(w, http.StatusOK, Video)
}
