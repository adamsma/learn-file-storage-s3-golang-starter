package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {

	// parse vidoe ID from query string
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	// check authentication
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

	// parse form with limited memory
	const maxMemory = 1 << 30
	r.ParseMultipartForm(maxMemory)

	// retrieve video meta data
	videoMeta, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable find video metadata", err)
		return
	}
	if videoMeta.CreateVideoParams.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Only video creator can upload its video", err)
		return
	}

	// load video data and check type
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header["Content-Type"][0])
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type", nil)
		return
	}

	// create temporary object to be able to upload
	asset, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file on server", err)
		return
	}
	defer os.Remove(asset.Name())
	defer asset.Close()

	if _, err = io.Copy(asset, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error saving file", err)
		return
	}

	// allow read file from beginning
	asset.Seek(0, io.SeekStart)

	// determine aspect raio
	ratio, err := getVideoAspectRatio(asset.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to determine aspect ratio", err)
		return
	}

	// encode for fast start
	fastPath, err := processVideoForFastStart(asset.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to encode for faststart", err)
		return
	}
	defer os.Remove(fastPath)

	fastFile, err := os.Open(fastPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to load faststart file", err)
		return
	}
	defer fastFile.Close()

	// upload to S3
	opts, _ := mime.ExtensionsByType(mediaType)
	ext := opts[0]
	if slices.Contains(opts, ".mp4") {
		ext = ".mp4"
	}

	randId := make([]byte, 32)
	_, err = rand.Read(randId)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file on server", err)
		return
	}

	assetKey := cfg.getS3Key(base64.RawURLEncoding.EncodeToString(randId), ratio, ext)

	params := s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(assetKey),
		Body:        fastFile,
		ContentType: aws.String(mediaType),
	}
	cfg.s3Client.PutObject(r.Context(), &params)

	// update metadata
	videoURL := cfg.getS3URL(assetKey)
	videoMeta.VideoURL = &videoURL
	err = cfg.db.UpdateVideo(videoMeta)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video metadata", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoMeta)

}
