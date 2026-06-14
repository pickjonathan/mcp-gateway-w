package invites

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// newToken mints a single-use accept token. The emitted token embeds the org so
// the public accept endpoint (which carries no org in its path) can resolve the
// right realm/RLS scope. Only the hash of the secret is stored.
func newToken(org string) (raw, hash string) {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	secret := base64.RawURLEncoding.EncodeToString(b)
	return org + "." + secret, hashSecret(secret)
}

func hashSecret(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

// parseToken splits "{org}.{secret}". Org slugs never contain '.', so the first
// dot is the separator.
func parseToken(raw string) (org, secret string, ok bool) {
	i := strings.IndexByte(raw, '.')
	if i <= 0 || i == len(raw)-1 {
		return "", "", false
	}
	return raw[:i], raw[i+1:], true
}
