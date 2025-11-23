package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	MediaType, _, err := mime.ParseMediaType(FileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if MediaType != "image/jpeg" && MediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	bts := make([]byte, 32)
	rand.Read(bts)
	rNameBase64 := base64.RawURLEncoding.EncodeToString(bts)
	SavePath := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%v.%s", rNameBase64, strings.Split(MediaType, "/")[1]))
	File, err := os.Create(SavePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create file", err)
		return
	}
	defer File.Close()
	io.Copy(File, FileData)
	FilePath := fmt.Sprintf("http://localhost:%v/%v", cfg.port, SavePath)
	Video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Not your video", err)
		return
	}
	Video.ThumbnailURL = &FilePath
	err = cfg.db.UpdateVideo(Video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}
	respondWithJSON(w, http.StatusOK, Video)
}
