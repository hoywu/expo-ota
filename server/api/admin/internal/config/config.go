// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	DataSource string // PostgreSQL DSN; resolved from $(DB_URL) via conf.UseEnv()
	Auth       struct {
		AccessSecret string
		AccessExpire int64
	}
	RefreshSecret string
	RefreshExpire int64
	Cos           struct {
		SecretID  string
		SecretKey string
		Region    string
		Bucket    string
		Domain    string `json:",optional"` // optional custom COS domain
		KeyPrefix string `json:",optional"` // optional COS object key prefix
	}
	// SigningKeyEncryptionKey is the base64-encoded 32-byte AES-256-GCM key
	// used to encrypt code signing private keys at rest (§5.5).
	SigningKeyEncryptionKey string
	// PresignExpireSeconds is the validity window of COS pre-signed PUT URLs.
	PresignExpireSeconds int64 `json:",default=900"`
}
