package logging

import (
	"bytes"
	"strings"
	"testing"

	"github.com/acme-corp/mcp-runtime/pkg/config"
)

// TestLoggerRedactsSecrets proves the wired logger scrubs a secret even when it
// is (mistakenly) passed as a structured field — the redaction is at the writer,
// not the call site (T080).
func TestLoggerRedactsSecrets(t *testing.T) {
	var buf bytes.Buffer
	lg := buildWithWriter(&config.Config{LogFormat: "json", Env: "test"}, &buf)

	lg.Info().
		Str("authorization", "Bearer s3cr3t").
		Str("user", "alice").
		Msg("downstream call")

	out := buf.String()
	if strings.Contains(out, "s3cr3t") {
		t.Fatalf("secret leaked into logs: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("authorization not redacted: %s", out)
	}
	if !strings.Contains(out, "alice") {
		t.Fatalf("non-sensitive field should be preserved: %s", out)
	}
}
