package main

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const MaxUploadSize = 1 << 30
	http.MaxBytesReader(w, r.Body, MaxUploadSize)
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
	Video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return
	}
	if Video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized", err)
		return
	}
	file, _, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get video file", err)
		return
	}
	defer file.Close()
	mediatype, _, err := mime.ParseMediaType("video/mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse media type", err)
		return
	}
	if mediatype != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
		return
	}
	File, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create temp file", err)
		return
	}
	defer File.Close()
	_, err = io.Copy(File, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't copy file", err)
		return
	}
	_, err = File.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't seek file", err)
		return
	}
	filePath := File.Name()
	asp, err := getVideoAspectRatio(filePath)
	if err != nil {
		log.Print(err)
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video aspect ratio", err)
		return
	}
	var orientation string
	switch asp {
	case "16:9":
		orientation = "landscape"
	case "9:16":
		orientation = "portrait"
	default:
		orientation = "portrait"
	}
	filePath, err = processVideoForFastStart(filePath)
	if err != nil {
		log.Print(err)
		respondWithError(w, http.StatusInternalServerError, "Couldn't faststart video", err)
		return
	}
	fileFF, err := os.Open(filePath)
	if err != nil {
		log.Print(err)
		respondWithError(w, http.StatusInternalServerError, "Couldn't open video file", err)
		return
	}
	randomFileName := orientation + "/" + uuid.New().String() + ".mp4"
	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &randomFileName,
		Body:        fileFF,
		ContentType: &mediatype,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload video", err)
		return
	}
	vurl := fmt.Sprintf("https://d1odgk8zxb1m93.cloudfront.net/%s", randomFileName)
	// vurl := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v", cfg.s3Bucket, cfg.s3Region, randomFileName)
	Video.VideoURL = &vurl
	err = cfg.db.UpdateVideo(Video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}
}
