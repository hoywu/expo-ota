package storage

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestCosStoreUploadReadDeleteWithEnv(t *testing.T) {
	cfg := cosConfigFromEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	store, err := NewCosStore(cfg)
	if err != nil {
		t.Fatalf("NewCosStore() error = %v", err)
	}

	cosStore, ok := store.(*cosStore)
	if !ok {
		t.Fatalf("NewCosStore() returned %T, want *cosStore", store)
	}

	SetAssetKeyPrefix(os.Getenv("COS_KEY_PREFIX"))
	t.Cleanup(func() {
		SetAssetKeyPrefix("")
	})

	randomText := randomHex(t, 16)
	storageKey := AssetStorageKey("integration-test", time.Now().UTC().Format("20060102T150405Z")+"-"+randomText+".txt")
	body := []byte("storage integration test\nrandom=" + randomText + "\n")
	bodyMD5 := md5.Sum(body)
	contentMD5 := base64.StdEncoding.EncodeToString(bodyMD5[:])

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if _, err := cosStore.client.Object.Delete(cleanupCtx, storageKey); err != nil {
			t.Logf("delete test object %q: %v", storageKey, err)
		}
	}()

	putURL, headers, err := store.PresignPut(ctx, storageKey, "text/plain; charset=utf-8", contentMD5, 5*time.Minute)
	if err != nil {
		t.Fatalf("PresignPut() error = %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT presigned URL error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		responseBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("PUT presigned URL status = %d, body = %s", resp.StatusCode, responseBody)
	}

	sizeBytes, err := store.Head(ctx, storageKey)
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}
	if sizeBytes != int64(len(body)) {
		t.Fatalf("Head() size = %d, want %d", sizeBytes, len(body))
	}

	getResp, err := cosStore.client.Object.Get(ctx, storageKey, nil)
	if err != nil {
		t.Fatalf("Object.Get() error = %v", err)
	}
	defer getResp.Body.Close()

	got, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("Object.Get() body = %q, want %q", got, body)
	}
}

func cosConfigFromEnv(t *testing.T) CosConfig {
	t.Helper()

	required := map[string]string{
		"COS_SECRET_ID":  os.Getenv("COS_SECRET_ID"),
		"COS_SECRET_KEY": os.Getenv("COS_SECRET_KEY"),
		"COS_REGION":     os.Getenv("COS_REGION"),
		"COS_BUCKET":     os.Getenv("COS_BUCKET"),
	}
	for key, value := range required {
		if value == "" {
			t.Skipf("%s is not set", key)
		}
	}

	return CosConfig{
		SecretID:  required["COS_SECRET_ID"],
		SecretKey: required["COS_SECRET_KEY"],
		Region:    required["COS_REGION"],
		Bucket:    required["COS_BUCKET"],
		Domain:    os.Getenv("COS_DOMAIN"),
	}
}

func randomHex(t *testing.T, size int) string {
	t.Helper()

	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}
	return hex.EncodeToString(buf)
}
