package workers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ObjectStoreConfig struct {
	Bucket       string
	Region       string
	Endpoint     string
	AccessKeyID  string
	SecretKey    string
	UsePathStyle bool
}

type ObjectStore interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Bucket() string
	Enabled() bool
}

type s3ObjectStore struct {
	client *s3.Client
	bucket string
}

func NewObjectStore(ctx context.Context, cfg ObjectStoreConfig) (ObjectStore, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return &disabledObjectStore{}, nil
	}

	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}

	if cfg.AccessKeyID != "" || cfg.SecretKey != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.RetryMaxAttempts = 3
		o.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	})

	return &s3ObjectStore{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

func (s *s3ObjectStore) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	if _, err := s.client.PutObject(ctx, input); err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

func (s *s3ObjectStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get object %s: %w", key, err)
	}
	return out.Body, nil
}

func (s *s3ObjectStore) Delete(ctx context.Context, key string) error {
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("delete object %s: %w", key, err)
	}
	return nil
}

func (s *s3ObjectStore) Bucket() string {
	return s.bucket
}

func (s *s3ObjectStore) Enabled() bool {
	return true
}

type disabledObjectStore struct{}

func (d *disabledObjectStore) Put(_ context.Context, _ string, _ io.Reader, _ int64, _ string) error {
	return nil
}

func (d *disabledObjectStore) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("object store disabled")
}

func (d *disabledObjectStore) Delete(_ context.Context, _ string) error {
	return nil
}

func (d *disabledObjectStore) Bucket() string {
	return ""
}

func (d *disabledObjectStore) Enabled() bool {
	return false
}
