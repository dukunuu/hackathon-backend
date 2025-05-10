package file

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type FileStore interface {
	UploadProfileImage(ctx context.Context, fileContent []byte, originalFilename string, contentType string, userID uuid.UUID) (objectURL string, objectName string, err error)
	UploadPostImage(ctx context.Context, fileContent []byte, originalFilename string, contentType string, userID uuid.UUID) (objectURL string, objectName string, err error)
	DeleteObject(ctx context.Context, objectName string) error // Optional: for rollbacks or deletions
}

type MinioStore struct {
	client        *minio.Client
	bucketName    string
	publicURLBase string // e.g., "http://localhost:9000" or "https://your-cdn.com"
}

type MinioConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	UseSSL          bool
	PublicURLBase   string
}

func NewMinioStore(cfg MinioConfig) (*MinioStore, error) {
	minioClient, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MinIO client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exists, err := minioClient.BucketExists(ctx, cfg.BucketName)
	if err != nil {
	   return nil, fmt.Errorf("failed to check if MinIO bucket exists: %w", err)
	}
	if !exists {
	   err = minioClient.MakeBucket(ctx, cfg.BucketName, minio.MakeBucketOptions{})
	   if err != nil {
	       return nil, fmt.Errorf("failed to create MinIO bucket '%s': %w", cfg.BucketName, err)
	   }
	}

	return &MinioStore{
		client:        minioClient,
		bucketName:    cfg.BucketName,
		publicURLBase: cfg.PublicURLBase,
	}, nil
}

// UploadProfileImage uploads a user's profile image to MinIO.
// It takes fileContent as []byte to simplify handling after initial read/validation in the handler.
func (s *MinioStore) UploadProfileImage(ctx context.Context, fileContent []byte, originalFilename string, contentType string, userID uuid.UUID) (objectURL string, objectName string, err error) {
	fileExt := filepath.Ext(originalFilename)
	if fileExt == "" { // Fallback if no extension
		switch contentType {
		case "image/jpeg": fileExt = ".jpg"
		case "image/png": fileExt = ".png"
		case "image/gif": fileExt = ".gif"
		case "image/webp": fileExt = ".webp"
		default: fileExt = ".img"
		}
	}

	objectName = fmt.Sprintf("user_profiles/%s/%d_%s%s",
		userID.String(),
		time.Now().UnixNano(),
		uuid.New().String(), // Another UUID for uniqueness within the timestamp
		fileExt,
	)

	fileReader := bytes.NewReader(fileContent)
	fileSize := int64(len(fileContent))

	_, err = s.client.PutObject(ctx, s.bucketName, objectName, fileReader, fileSize, minio.PutObjectOptions{
		ContentType: contentType,
		// UserMetadata: map[string]string{"original-filename": originalFilename},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to upload object to MinIO (bucket: %s, object: %s): %w", s.bucketName, objectName, err)
	}

	objectURL = fmt.Sprintf("%s/%s/%s", s.publicURLBase, s.bucketName, objectName)
	return objectURL, objectName, nil
}

func (s *MinioStore) DeleteObject(ctx context.Context, objectName string) error {
	err := s.client.RemoveObject(ctx, s.bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to remove object '%s' from bucket '%s': %w", objectName, s.bucketName, err)
	}
	return nil
}

