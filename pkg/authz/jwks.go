package authz

import (
	"context"
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// StaticKeySource serves a fixed set of public keys by kid (used in tests and
// for pinned keys). A key registered under "" acts as a fallback.
type StaticKeySource struct{ Keys map[string]crypto.PublicKey }

// KeyFor implements KeySource.
func (s StaticKeySource) KeyFor(_ context.Context, _ string, kid string) (crypto.PublicKey, error) {
	if k, ok := s.Keys[kid]; ok {
		return k, nil
	}
	if k, ok := s.Keys[""]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("authz: no key for kid %q", kid)
}

// JWKSKeySource fetches and caches JWKS documents per issuer (RSA keys). Keycloak
// publishes its JWKS at "{issuer}/protocol/openid-connect/certs".
type JWKSKeySource struct {
	client *http.Client
	ttl    time.Duration

	mu    sync.Mutex
	cache map[string]*jwksEntry // issuer -> entry
}

type jwksEntry struct {
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
}

// NewJWKSKeySource returns a JWKS-backed key source with a 10-minute cache TTL.
func NewJWKSKeySource() *JWKSKeySource {
	return &JWKSKeySource{
		client: &http.Client{Timeout: 5 * time.Second},
		ttl:    10 * time.Minute,
		cache:  map[string]*jwksEntry{},
	}
}

// KeyFor implements KeySource, refreshing the issuer's JWKS on a cache miss.
func (j *JWKSKeySource) KeyFor(ctx context.Context, issuer, kid string) (crypto.PublicKey, error) {
	if k := j.cached(issuer, kid); k != nil {
		return k, nil
	}
	if err := j.refresh(ctx, issuer); err != nil {
		return nil, err
	}
	if k := j.cached(issuer, kid); k != nil {
		return k, nil
	}
	return nil, fmt.Errorf("authz: key %q not found for issuer %s", kid, issuer)
}

func (j *JWKSKeySource) cached(issuer, kid string) *rsa.PublicKey {
	j.mu.Lock()
	defer j.mu.Unlock()
	e, ok := j.cache[issuer]
	if !ok || time.Since(e.fetchedAt) > j.ttl {
		return nil
	}
	return e.keys[kid]
}

func (j *JWKSKeySource) refresh(ctx context.Context, issuer string) error {
	url := issuer + "/protocol/openid-connect/certs"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := j.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authz: jwks fetch %s: status %d", url, resp.StatusCode)
	}

	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}

	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := rsaPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	j.mu.Lock()
	j.cache[issuer] = &jwksEntry{keys: keys, fetchedAt: time.Now()}
	j.mu.Unlock()
	return nil
}

func rsaPublicKey(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}
