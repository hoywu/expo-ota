package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

// CosConfig carries the Tencent Cloud COS credentials and bucket location.
type CosConfig struct {
	SecretID  string
	SecretKey string
	Region    string
	Bucket    string
	Domain    string // optional custom COS domain (host only or with scheme)
}

type cosStore struct {
	client    *cos.Client
	cfg       CosConfig
	publicURL *url.URL
}

// NewCosStore builds a Store backed by Tencent Cloud COS.
func NewCosStore(cfg CosConfig) (Store, error) {
	bucketURL, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", cfg.Bucket, cfg.Region))
	if err != nil {
		return nil, err
	}

	endpointURL := bucketURL
	publicURL := bucketURL
	if cfg.Domain != "" {
		domain := cfg.Domain
		if !strings.Contains(domain, "://") {
			domain = "https://" + domain
		}
		customURL, err := url.Parse(domain)
		if err != nil {
			return nil, err
		}
		endpointURL = customURL
		publicURL = customURL
	}

	client := cos.NewClient(&cos.BaseURL{BucketURL: endpointURL}, &http.Client{
		Timeout: 30 * time.Second,
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
	})

	return &cosStore{client: client, cfg: cfg, publicURL: publicURL}, nil
}

func (s *cosStore) PublicURL(storageKey string) string {
	return s.publicURL.JoinPath(storageKey).String()
}

func (s *cosStore) PresignPut(ctx context.Context, storageKey, contentType string, expires time.Duration) (string, map[string]string, error) {
	presigned, err := s.client.Object.GetPresignedURL(
		ctx, http.MethodPut, storageKey, s.cfg.SecretID, s.cfg.SecretKey, expires, nil)
	if err != nil {
		return "", nil, err
	}

	return presigned.String(), map[string]string{"Content-Type": contentType}, nil
}

func (s *cosStore) Head(ctx context.Context, storageKey string) (int64, error) {
	resp, err := s.client.Object.Head(ctx, storageKey, nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.ContentLength, nil
}
