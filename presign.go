package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)

	request, err := presignClient.PresignGetObject(context.Background(),
		&s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &key,
		},
		s3.WithPresignExpires(expireTime),
	)
	if err != nil {
		return "", err
	}

	return request.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}

	var bucket, key string
	urlStr := *video.VideoURL

	// Check if it's already in bucket,key format
	if parts := strings.Split(urlStr, ","); len(parts) == 2 {
		bucket, key = parts[0], parts[1]
	} else if strings.Contains(urlStr, ".amazonaws.com/") {
		// Parse legacy S3 URL format
		parts := strings.Split(urlStr, ".amazonaws.com/")
		if len(parts) != 2 {
			return video, fmt.Errorf("invalid video URL format")
		}
		bucketParts := strings.Split(parts[0], "//")
		if len(bucketParts) != 2 {
			return video, fmt.Errorf("invalid video URL format")
		}
		bucket = bucketParts[1]
		key = parts[1]
	} else {
		return video, fmt.Errorf("invalid video URL format")
	}

	// Generate presigned URL
	signedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, time.Hour)
	if err != nil {
		return video, err
	}

	signedVideo := video
	signedVideo.VideoURL = &signedURL
	return signedVideo, nil
}
