package main

import (
	"context"
	"log"
	"log/slog"

	"github.com/dukunuu/hackathon_backend/ai"
	"github.com/dukunuu/hackathon_backend/config"
	"github.com/dukunuu/hackathon_backend/db"
	_ "github.com/dukunuu/hackathon_backend/docs"
	"github.com/dukunuu/hackathon_backend/file"
	"github.com/dukunuu/hackathon_backend/server"
)

// @title Hackathon Backend API
// @version 1.0
// @description This is a sample server for a hackathon backend.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description "Type 'Bearer YOUR_JWT_TOKEN' to authorize."
func main() {
	ctx := context.Background()
	cfg, err := config.LoadConfig()	
	if err!=nil {
		log.Fatal("Failed to load config")
	}

	db, err := db.Init(cfg.DB_URL, &ctx)
	if err!=nil {
		log.Fatal("Failed to load config")
	}

	fileCfg := file.MinioConfig{
		Endpoint: cfg.MINIO_ENDPOINT,
		AccessKeyID: cfg.MINIO_ACCESS_KEY_ID,
		SecretAccessKey: cfg.MINIO_SECRET_ACCESS_KEY,
		BucketName: cfg.MINIO_BUCKET_NAME,
		UseSSL: cfg.MINIO_USE_SSL,
		PublicURLBase: cfg.MINIO_PUBLIC_URL_BASE,
	}

	store, err := file.NewMinioStore(fileCfg)
	if err!=nil {
		log.Fatal("Failed to load file store: ", err)
	}

	aiModel, err := ai.Init(cfg)
	if err!=nil {
		log.Fatal("Failed to load file store: ", err)
	}

	if aiModel != nil {
    slog.Info("Sending warm-up request to Ollama model", "model", cfg.OLLAMA_MODEL_NAME)
    // Use a very simple prompt that the model should easily handle
    warmUpPrompt := "Hello! Are you ready?"
    _, err := aiModel.GenerateResponse(warmUpPrompt)
    if err != nil {
        slog.Warn("Ollama warm-up request failed (this might be okay if it's still loading)", "error", err)
    } else {
        slog.Info("Ollama warm-up request successful.")
    }
}

	srvr := server.Init(cfg.HOST, cfg.JWT_SECRET, db, store, aiModel)

	log.Printf("Starting server on port: %s", cfg.HOST)
	srvr.Start()
}
