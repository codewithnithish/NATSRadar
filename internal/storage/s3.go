package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// S3Driver communicates with AWS S3, MinIO, Cloudflare R2, Wasabi, or Ceph via standard S3 REST API.
type S3Driver struct {
	endpoint  string
	bucket    string
	region    string
	accessKey string
	secretKey string
	useSSL    bool
	client    *http.Client
}

// NewS3 creates a driver connected to an S3-compatible object store.
func NewS3(endpoint, bucket, region, accessKey, secretKey string, useSSL bool) *S3Driver {
	if region == "" {
		region = "us-east-1"
	}
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	return &S3Driver{
		endpoint:  endpoint,
		bucket:    bucket,
		region:    region,
		accessKey: accessKey,
		secretKey: secretKey,
		useSSL:    useSSL,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *S3Driver) Name() string {
	return "s3"
}

func (s *S3Driver) getURL(p string) string {
	scheme := "https"
	if !s.useSSL {
		scheme = "http"
	}
	p = strings.TrimPrefix(p, "/")
	return fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpoint, s.bucket, p)
}

func (s *S3Driver) Load(ctx context.Context, p string) ([]byte, error) {
	reqURL := s.getURL(p)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create s3 get req: %w", err)
	}

	s.signRequest(req, nil)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("s3 get %s: %w", p, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("s3 get status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (s *S3Driver) Save(ctx context.Context, p string, data []byte) error {
	reqURL := s.getURL(p)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, reqURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create s3 put req: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	s.signRequest(req, data)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("s3 put %s: %w", p, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("s3 put status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (s *S3Driver) Delete(ctx context.Context, p string) error {
	reqURL := s.getURL(p)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create s3 del req: %w", err)
	}

	s.signRequest(req, nil)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("s3 delete %s: %w", p, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("s3 delete status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (s *S3Driver) List(ctx context.Context, prefix string) ([]string, error) {
	// Simple prefix check or head
	return nil, nil
}

func (s *S3Driver) Close() error {
	s.client.CloseIdleConnections()
	return nil
}

// AWS SigV4 implementation for standard zero-dependency S3 authentication
func (s *S3Driver) signRequest(req *http.Request, payload []byte) {
	if s.accessKey == "" || s.secretKey == "" {
		return // Anonymous access (e.g. public MinIO bucket)
	}

	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	payloadHash := sha256Hex(payload)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("Host", req.URL.Host)

	// Canonical Request
	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := req.URL.RawQuery

	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.URL.Host, payloadHash, amzDate)
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	// String to Sign
	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, s.region)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	// Signing Key
	kDate := hmacSHA256([]byte("AWS4"+s.secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(s.region))
	kService := hmacSHA256(kRegion, []byte("s3"))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))

	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
