package authz

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func mint(t *testing.T, key *rsa.PrivateKey, iss, aud, sub string, exp time.Time, roles []string) string {
	t.Helper()
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    iss,
			Subject:   sub,
			Audience:  jwt.ClaimStrings{aud},
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	c.RealmAccess.Roles = roles
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
	tok.Header["kid"] = "test"
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func newValidator(key *rsa.PrivateKey) *JWTValidator {
	ks := StaticKeySource{Keys: map[string]crypto.PublicKey{"test": &key.PublicKey}}
	return NewJWTValidator("mcp.example.com", "https://auth.mcp.example.com/realms/%s", ks)
}

const (
	acmeIss = "https://auth.mcp.example.com/realms/acme"
	acmeAud = "https://acme.mcp.example.com/mcp"
)

func TestValidate_OK(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newValidator(key)
	tok := mint(t, key, acmeIss, acmeAud, "user-1", time.Now().Add(time.Hour), []string{"engineers"})

	p, err := v.Validate(context.Background(), tok, "acme.mcp.example.com")
	if err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
	if p.OrgID != "acme" || p.UserID != "user-1" || len(p.Roles) != 1 || p.Roles[0] != "engineers" {
		t.Fatalf("unexpected principal: %+v", p)
	}
}

func TestValidate_CrossOrgAudienceRejected(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newValidator(key)
	// Token minted for acme, presented at beta's endpoint -> must be rejected (HC-1).
	tok := mint(t, key, acmeIss, acmeAud, "user-1", time.Now().Add(time.Hour), nil)
	if _, err := v.Validate(context.Background(), tok, "beta.mcp.example.com"); err == nil {
		t.Fatal("expected rejection: acme token used against beta endpoint")
	}
}

func TestValidate_Expired(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newValidator(key)
	tok := mint(t, key, acmeIss, acmeAud, "user-1", time.Now().Add(-time.Minute), nil)
	if _, err := v.Validate(context.Background(), tok, "acme.mcp.example.com"); err == nil {
		t.Fatal("expected expired token rejection")
	}
}

func TestValidate_BadSignature(t *testing.T) {
	trusted, _ := rsa.GenerateKey(rand.Reader, 2048)
	attacker, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newValidator(trusted) // gateway trusts `trusted`
	tok := mint(t, attacker, acmeIss, acmeAud, "user-1", time.Now().Add(time.Hour), nil)
	if _, err := v.Validate(context.Background(), tok, "acme.mcp.example.com"); err == nil {
		t.Fatal("expected bad-signature rejection")
	}
}

func TestValidate_UnknownHost(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := newValidator(key)
	tok := mint(t, key, acmeIss, acmeAud, "user-1", time.Now().Add(time.Hour), nil)
	if _, err := v.Validate(context.Background(), tok, "example.org"); err == nil {
		t.Fatal("expected unknown-host rejection")
	}
}
