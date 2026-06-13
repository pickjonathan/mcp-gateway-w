// Package config loads all runtime configuration from environment variables
// into a single Config value that is shared process-wide as a singleton via Get.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config holds all runtime configuration. It is populated once from the
// environment and shared across the application as a singleton (see Get).
type Config struct {
	// Core
	Env             string        // dev | staging | prod
	HTTPAddr        string        // gateway listen address
	BaseDomain      string        // e.g. mcp.example.com -> {org}.mcp.example.com/mcp
	ShutdownTimeout time.Duration

	// Logging
	LogLevel  string // debug | info | warn | error
	LogFormat string // json | console

	// Datastores
	PostgresDSN string
	RedisAddr   string
	VaultAddr   string
	VaultToken  string

	// Auth (Keycloak). KeycloakIssuerTemplate takes the org slug, e.g.
	// "https://auth.mcp.example.com/realms/%s".
	KeycloakIssuerTemplate string

	// AdminAudience is the expected token audience for the control-plane admin API.
	AdminAudience string

	// SandboxRuntime selects the untrusted-execution backend: gvisor | kata | runc | exec (dev).
	SandboxRuntime string

	// SandboxImage is the base container image stdio servers run in (node/npx, python/uv).
	SandboxImage string

	// Per-minute request limits (0 = unlimited) — noisy-neighbor protection (FR-017).
	RateOrgPerMin  int
	RateUserPerMin int

	// OTLPEndpoint is the OpenTelemetry OTLP/HTTP traces endpoint (host:port). Empty
	// disables span export (spans are still created but dropped). FR-008.
	OTLPEndpoint string

	// Durable audit archive (S3-compatible, Object-Lock'd; T087). An empty
	// endpoint selects the in-memory dev logger.
	AuditS3Endpoint  string
	AuditS3AccessKey string
	AuditS3SecretKey string
	AuditS3Bucket    string
	AuditS3UseSSL    bool
	AuditRetention   time.Duration

	// BlockPrivateEgress refuses gateway connections to non-public IPs
	// (loopback/private/link-local/metadata) — SSRF protection for admin-supplied
	// remote MCP endpoints. Defaults on in prod, off in dev (loopback test servers).
	BlockPrivateEgress bool

	// AuditDenyPerMin bounds denial-audit writes per minute (0 = unlimited), so
	// unauthenticated request floods can't amplify audit storage. Drops are
	// counted in the mcp_audit_dropped_total metric.
	AuditDenyPerMin int
}

var (
	instance *Config
	once     sync.Once
)

// Get returns the process-wide Config singleton, loading it from the
// environment on first call. Subsequent calls return the same instance.
func Get() *Config {
	once.Do(func() { instance = load() })
	return instance
}

// load reads configuration from environment variables (prefix MCP_), applying
// defaults suitable for local development.
func load() *Config {
	env := getEnv("MCP_ENV", "dev")
	return &Config{
		Env:                    env,
		HTTPAddr:               getEnv("MCP_HTTP_ADDR", ":8080"),
		BaseDomain:             getEnv("MCP_BASE_DOMAIN", "mcp.example.com"),
		ShutdownTimeout:        getDuration("MCP_SHUTDOWN_TIMEOUT", 15*time.Second),
		LogLevel:               getEnv("MCP_LOG_LEVEL", "info"),
		LogFormat:              getEnv("MCP_LOG_FORMAT", "console"),
		PostgresDSN:            getEnv("MCP_POSTGRES_DSN", ""),
		RedisAddr:              getEnv("MCP_REDIS_ADDR", "localhost:6379"),
		VaultAddr:              getEnv("MCP_VAULT_ADDR", ""),
		VaultToken:             getEnv("MCP_VAULT_TOKEN", ""),
		KeycloakIssuerTemplate: getEnv("MCP_KEYCLOAK_ISSUER_TEMPLATE", "https://auth.mcp.example.com/realms/%s"),
		AdminAudience:          getEnv("MCP_ADMIN_AUDIENCE", "https://api.mcp.example.com"),
		SandboxRuntime:         getEnv("MCP_SANDBOX_RUNTIME", "gvisor"),
		SandboxImage:           getEnv("MCP_SANDBOX_IMAGE", "acme/mcp-sandbox:dev"),
		RateOrgPerMin:          getInt("MCP_RATE_ORG_PER_MIN", 0),
		RateUserPerMin:         getInt("MCP_RATE_USER_PER_MIN", 0),
		OTLPEndpoint:           getEnv("MCP_OTLP_ENDPOINT", ""),
		AuditS3Endpoint:        getEnv("MCP_AUDIT_S3_ENDPOINT", ""),
		AuditS3AccessKey:       getEnv("MCP_AUDIT_S3_ACCESS_KEY", ""),
		AuditS3SecretKey:       getEnv("MCP_AUDIT_S3_SECRET_KEY", ""),
		AuditS3Bucket:          getEnv("MCP_AUDIT_S3_BUCKET", "mcp-audit"),
		AuditS3UseSSL:          getBool("MCP_AUDIT_S3_USE_SSL", false),
		AuditRetention:         getDuration("MCP_AUDIT_RETENTION", 365*24*time.Hour),
		BlockPrivateEgress:     getBool("MCP_BLOCK_PRIVATE_EGRESS", strings.EqualFold(env, "prod")),
		AuditDenyPerMin:        getInt("MCP_AUDIT_DENY_PER_MIN", 600),
	}
}

// IsProd reports whether the runtime environment is production.
func (c *Config) IsProd() bool { return strings.EqualFold(c.Env, "prod") }

// Validate checks that required configuration is present. It is permissive in
// dev and strict in prod.
func (c *Config) Validate() error {
	if !c.IsProd() {
		return nil
	}
	var missing []string
	if c.PostgresDSN == "" {
		missing = append(missing, "MCP_POSTGRES_DSN")
	}
	if c.VaultAddr == "" {
		missing = append(missing, "MCP_VAULT_ADDR")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config in prod: %s", strings.Join(missing, ", "))
	}
	return nil
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
