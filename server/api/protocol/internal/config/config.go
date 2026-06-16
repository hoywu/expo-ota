// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	DataSource string // PostgreSQL DSN; resolved from $(DB_URL) via conf.UseEnv()
	// SigningKeyEncryptionKey is the base64-encoded 32-byte AES-256-GCM key
	// used to decrypt code signing private keys at rest (§5.5). The protocol
	// service only signs manifests; it never writes keys.
	SigningKeyEncryptionKey string
}
