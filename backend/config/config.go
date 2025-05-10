// config/config.go
package config

import (
	"fmt"
	"github.com/dukunuu/hackathon_backend/common" // Assuming this path is correct
)

type Config struct {
	DB_URL               string
	HOST                 string
	JWT_SECRET           string
	MINIO_ENDPOINT       string
	MINIO_ACCESS_KEY_ID  string
	MINIO_SECRET_ACCESS_KEY string
	MINIO_BUCKET_NAME    string
	MINIO_USE_SSL        bool
	MINIO_PUBLIC_URL_BASE string // Optional: If your MinIO access is behind a different public URL/CDN
}


func LoadConfig() (*Config, error) {
	dbUrl := common.GetString("DB_URL", "")
	if dbUrl == "" {
		return nil, fmt.Errorf("could not load env DB_URL")
	}

	addr := common.GetString("HOST", ":8080") // Defaulting to 8000 as per previous discussion
	jwt := common.GetString("JWT_SECRET", "my_app_secret")

	minioEndpoint := common.GetString("MINIO_ENDPOINT", "")
	minioAccessKey := common.GetString("MINIO_ACCESS_KEY_ID", "")
	minioSecretKey := common.GetString("MINIO_SECRET_ACCESS_KEY", "")
	minioBucket := common.GetString("MINIO_BUCKET_NAME", "")
	minioUseSSL := common.GetBool("MINIO_USE_SSL", false)
	minioPublicURLBase := common.GetString("MINIO_PUBLIC_URL_BASE", "")


	if minioEndpoint == "" || minioAccessKey == "" || minioSecretKey == "" || minioBucket == "" {
		return nil, fmt.Errorf("MINIO_ENDPOINT, MINIO_ACCESS_KEY_ID, MINIO_SECRET_ACCESS_KEY, and MINIO_BUCKET_NAME must be set")
	}

	// If MINIO_PUBLIC_URL_BASE is not set, construct it from endpoint and SSL
	if minioPublicURLBase == "" {
		scheme := "http"
		if minioUseSSL {
			scheme = "https"
		}
		minioPublicURLBase = fmt.Sprintf("%s://%s", scheme, minioEndpoint)
	}
    minioPublicURLBase = common.TrimSuffix(minioPublicURLBase, "/") // Assuming common.TrimSuffix exists

	return &Config{
		DB_URL:                  dbUrl,
		HOST:                    addr,
		JWT_SECRET:              jwt,
		MINIO_ENDPOINT:          minioEndpoint,
		MINIO_ACCESS_KEY_ID:     minioAccessKey,
		MINIO_SECRET_ACCESS_KEY: minioSecretKey,
		MINIO_BUCKET_NAME:       minioBucket,
		MINIO_USE_SSL:           minioUseSSL,
		MINIO_PUBLIC_URL_BASE:   minioPublicURLBase,
	}, nil
}

