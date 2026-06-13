package telemetry

import (
	"fmt"
	"io"
	"regexp"
)

// Redacted is the placeholder substituted for sensitive values.
const Redacted = "[REDACTED]"

// sensitiveWords are matched (case-insensitively) inside field/header/env names;
// a name containing any of them has its value redacted. Kept deliberately broad —
// over-redacting a log line is safe, leaking a credential is not. "credential" is
// intentionally absent so the non-secret credential_mode field stays readable.
const sensitiveWords = `authorization|bearer|token|secret|password|passwd|api[_-]?key|cookie|private[_-]?key`

var (
	// "key":"value" where key contains a sensitive word (JSON string fields).
	reJSONString = regexp.MustCompile(`(?i)"([^"\\]*(?:` + sensitiveWords + `)[^"\\]*)"(\s*:\s*)"(?:[^"\\]|\\.)*"`)
	// "key":[ ... ] where key contains a sensitive word (e.g. http.Header arrays).
	reJSONArray = regexp.MustCompile(`(?i)"([^"\\]*(?:` + sensitiveWords + `)[^"\\]*)"(\s*:\s*)\[[^\]]*\]`)
	// key=value env entries / query params where key contains a sensitive word.
	reKeyValue = regexp.MustCompile(`(?i)([\w.\-]*(?:` + sensitiveWords + `)[\w.\-]*)=([^\s"&]+)`)
	// Bearer <token> appearing anywhere (e.g. inside a free-text message).
	reBearer = regexp.MustCompile(`(?i)(bearer\s+)[\w.\-+/=~]+`)

	replString = fmt.Sprintf(`"${1}"${2}"%s"`, Redacted)
	replArray  = fmt.Sprintf(`"${1}"${2}["%s"]`, Redacted)
	replKV     = fmt.Sprintf(`${1}=%s`, Redacted)
	replBearer = fmt.Sprintf(`${1}%s`, Redacted)
)

// Redact removes sensitive values (auth headers, API keys, tokens, env secrets,
// bearer tokens) from s, leaving the surrounding structure intact. It is a
// best-effort defense-in-depth layer for logs and trace attributes — code should
// still avoid logging secrets in the first place. Free-text secrets that aren't
// under a sensitive key and aren't bearer/key=value shaped are not caught.
func Redact(s string) string {
	s = reJSONString.ReplaceAllString(s, replString)
	s = reJSONArray.ReplaceAllString(s, replArray)
	s = reKeyValue.ReplaceAllString(s, replKV)
	s = reBearer.ReplaceAllString(s, replBearer)
	return s
}

// RedactingWriter wraps W and applies Redact to every write before forwarding —
// so secrets never reach the log sink regardless of how a field was added.
type RedactingWriter struct{ W io.Writer }

// Write redacts p and forwards it, reporting the original length so callers (e.g.
// zerolog) see a complete write even though the forwarded bytes differ.
func (rw RedactingWriter) Write(p []byte) (int, error) {
	if _, err := rw.W.Write([]byte(Redact(string(p)))); err != nil {
		return 0, err
	}
	return len(p), nil
}
