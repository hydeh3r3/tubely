package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	db               database.Client
	jwtSecret        string
	platform         string
	filepathRoot     string
	assetsRoot       string
	s3Client         *s3.Client
	s3Bucket         string
	s3Region         string
	s3CfDistribution string
	port             string
}

type thumbnail struct {
	data      []byte
	mediaType string
}

var videoThumbnails = map[uuid.UUID]thumbnail{}

func main() {
	godotenv.Load(".env")

	pathToDB := os.Getenv("DB_PATH")
	if pathToDB == "" {
		log.Fatal("DB_URL must be set")
	}

	db, err := database.NewClient(pathToDB)
	if err != nil {
		log.Fatalf("Couldn't connect to database: %v", err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is not set")
	}

	platform := os.Getenv("PLATFORM")
	if platform == "" {
		log.Fatal("PLATFORM environment variable is not set")
	}

	filepathRoot := os.Getenv("FILEPATH_ROOT")
	if filepathRoot == "" {
		log.Fatal("FILEPATH_ROOT environment variable is not set")
	}

	assetsRoot := os.Getenv("ASSETS_ROOT")
	if assetsRoot == "" {
		log.Fatal("ASSETS_ROOT environment variable is not set")
	}

	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		log.Fatal("S3_BUCKET environment variable is not set")
	}

	s3Region := os.Getenv("S3_REGION")
	if s3Region == "" {
		log.Fatal("S3_REGION environment variable is not set")
	}

	s3CfDistribution := os.Getenv("S3_CF_DISTRIBUTION")
	if s3CfDistribution == "" {
		log.Fatal("S3_CF_DISTRIBUTION environment variable is not set")
	}

	port := os.Getenv("PORT")
	if port == "" {
		log.Fatal("PORT environment variable is not set")
	}

	// Configure AWS S3 client
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(s3Region),
	)
	if err != nil {
		log.Fatal("Unable to load AWS SDK config:", err)
	}

	s3Client := s3.NewFromConfig(cfg)

	apiCfg := apiConfig{
		db:               db,
		jwtSecret:        jwtSecret,
		platform:         platform,
		filepathRoot:     filepathRoot,
		assetsRoot:       assetsRoot,
		s3Client:         s3Client,
		s3Bucket:         s3Bucket,
		s3Region:         s3Region,
		s3CfDistribution: s3CfDistribution,
		port:             port,
	}

	err = apiCfg.ensureAssetsDir()
	if err != nil {
		log.Fatalf("Couldn't create assets directory: %v", err)
	}

	mux := http.NewServeMux()
	appHandler := http.StripPrefix("/app", http.FileServer(http.Dir(filepathRoot)))
	mux.Handle("/app/", appHandler)

	assetsHandler := http.StripPrefix("/assets", http.FileServer(http.Dir(assetsRoot)))
	mux.Handle("/assets/", noCacheMiddleware(assetsHandler))

	mux.HandleFunc("POST /api/login", apiCfg.handlerLogin)
	mux.HandleFunc("POST /api/refresh", apiCfg.handlerRefresh)
	mux.HandleFunc("POST /api/revoke", apiCfg.handlerRevoke)

	mux.HandleFunc("POST /api/users", apiCfg.handlerUsersCreate)

	mux.HandleFunc("POST /api/videos", apiCfg.handlerVideoMetaCreate)
	mux.HandleFunc("POST /api/video_upload/{videoID}", apiCfg.handlerUploadVideo)
	mux.HandleFunc("GET /api/videos", apiCfg.handlerVideosRetrieve)
	mux.HandleFunc("GET /api/videos/{videoID}", apiCfg.handlerVideoGet)
	mux.HandleFunc("DELETE /api/videos/{videoID}", apiCfg.handlerVideoMetaDelete)
	mux.HandleFunc("POST /api/videos/{videoID}/thumbnail", apiCfg.handlerUploadThumbnail)

	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)

	mux.HandleFunc("GET /api/thumbnails/{videoID}", apiCfg.handlerThumbnailGet)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Serving on: http://localhost:%s/app/\n", port)
	log.Fatal(srv.ListenAndServe())
}

// processVideoForFastStart takes a file path and creates a new MP4 with fast start encoding
func processVideoForFastStart(filePath string) (string, error) {
	// Create output file path
	outputPath := filePath + ".processing"

	// Create ffmpeg command for fast start processing
	cmd := exec.Command(
		"ffmpeg",
		"-i", filePath,
		"-c", "copy",
		"-movflags", "faststart",
		"-f", "mp4",
		outputPath,
	)

	// Run the command
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to process video for fast start: %w", err)
	}

	return outputPath, nil
}
