package idp

import (
	"context"
	"net/http"
	"net/url"
)

// Broker configures per-realm identity brokering (OIDC/SAML) via the Admin API
// (US4 brokering). Kept separate from Keycloak so provisioning fakes need not
// implement it.
type Broker interface {
	UpsertIdentityProvider(ctx context.Context, realm string, idp IdentityProvider) error
	DeleteIdentityProvider(ctx context.Context, realm, alias string) error
}

// IdentityProvider is a brokered external IdP. Config carries provider settings
// including the (secret) clientSecret at apply time; it is never persisted in the
// control-plane DB (only a Vault ref is) and never logged.
type IdentityProvider struct {
	Alias      string
	ProviderID string // "oidc" | "saml"
	Enabled    bool
	Config     map[string]string
}

var _ Broker = (*RESTClient)(nil)

// UpsertIdentityProvider creates the IdP instance, or updates it if it exists.
func (c *RESTClient) UpsertIdentityProvider(ctx context.Context, realm string, ip IdentityProvider) error {
	body := map[string]any{
		"alias":      ip.Alias,
		"providerId": ip.ProviderID,
		"enabled":    ip.Enabled,
		"config":     ip.Config,
	}
	resp, err := c.do(ctx, http.MethodPost, "/admin/realms/"+url.PathEscape(realm)+"/identity-provider/instances", body)
	if err != nil {
		return err
	}
	resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusCreated:
		return nil
	case http.StatusConflict:
		// Update existing instance.
		resp2, err := c.do(ctx, http.MethodPut,
			"/admin/realms/"+url.PathEscape(realm)+"/identity-provider/instances/"+url.PathEscape(ip.Alias), body)
		if err != nil {
			return err
		}
		defer resp2.Body.Close()
		if resp2.StatusCode != http.StatusNoContent && resp2.StatusCode != http.StatusOK {
			return apiErr("update identity provider", resp2)
		}
		return nil
	default:
		return apiErr("create identity provider", resp)
	}
}

// DeleteIdentityProvider removes a brokered IdP instance.
func (c *RESTClient) DeleteIdentityProvider(ctx context.Context, realm, alias string) error {
	resp, err := c.do(ctx, http.MethodDelete,
		"/admin/realms/"+url.PathEscape(realm)+"/identity-provider/instances/"+url.PathEscape(alias), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return apiErr("delete identity provider", resp)
	}
	return nil
}
