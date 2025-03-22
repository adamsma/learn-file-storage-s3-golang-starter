package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func generatePresignedURL(
	s3Client *s3.Client, bucket, key string, expireTime time.Duration,
) (string, error) {

	params := s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	client := s3.NewPresignClient(s3Client)
	req, err := client.PresignGetObject(
		context.Background(),
		&params,
		s3.WithPresignExpires(expireTime),
	)

	if err != nil {
		return "", err
	}

	return req.URL, nil

}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {

	log.Printf("OG 'Video URL': %v", *video.VideoURL)

	params := strings.SplitN(*video.VideoURL, ",", 2)

	psURL, err := generatePresignedURL(cfg.s3Client, params[0], params[1], 5*time.Minute)
	if err != nil {
		return database.Video{}, err
	}

	video.VideoURL = &psURL

	return video, nil
}
