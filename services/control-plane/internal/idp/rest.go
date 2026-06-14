package idp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// RESTClient implements Keycloak against the Keycloak Admin REST API using a
// client-credentials service account. It caches the admin token until shortly
// before expiry. It holds no secret beyond what it is constructed with and never
// logs token/secret values.
type RESTClient struct {
	base         string // Keycloak base URL, no trailing slash
	adminRealm   string // realm the service-account client lives in (e.g. "master")
	clientID     string
	clientSecret string
	hc           *http.Client

	mu    sync.Mutex
	token string
	exp   time.Time
}

// NewRESTClient builds a client. adminRealm is where the privileged service-account
// client lives (typically "master"). clientSecret is resolved by the caller from
// the secret store (never read from env here).
func NewRESTClient(base, adminRealm, clientID, clientSecret string) *RESTClient {
	return &RESTClient{
		base:         strings.TrimRight(base, "/"),
		adminRealm:   adminRealm,
		clientID:     clientID,
		clientSecret: clientSecret,
		hc:           &http.Client{Timeout: 30 * time.Second},
	}
}

var _ Keycloak = (*RESTClient)(nil)

func (c *RESTClient) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.exp) {
		return c.token, nil
	}
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
	}
	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", c.base, url.PathEscape(c.adminRealm))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", apiErr("admin token", resp)
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", err
	}
	c.token = tr.AccessToken
	ttl := tr.ExpiresIn - 30
	if ttl < 5 {
		ttl = 5
	}
	c.exp = time.Now().Add(time.Duration(ttl) * time.Second)
	return c.token, nil
}

// do performs an authenticated Admin API request; body is JSON-encoded if non-nil.
func (c *RESTClient) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	tok, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.hc.Do(req)
}

func apiErr(op string, resp *http.Response) error {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return fmt.Errorf("keycloak %s: status %d: %s", op, resp.StatusCode, strings.TrimSpace(string(b)))
}

func idFromLocation(resp *http.Response) string {
	loc := resp.Header.Get("Location")
	if i := strings.LastIndex(loc, "/"); i >= 0 {
		return loc[i+1:]
	}
	return ""
}

func realmRep(r Realm) map[string]any {
	m := map[string]any{"realm": r.Name, "enabled": r.Enabled}
	if r.AccessTokenLifespan > 0 {
		m["accessTokenLifespan"] = r.AccessTokenLifespan
	}
	if r.SSOIdle > 0 {
		m["ssoSessionIdleTimeout"] = r.SSOIdle
	}
	if r.SSOMax > 0 {
		m["ssoSessionMaxLifespan"] = r.SSOMax
	}
	if r.SSLRequired != "" {
		m["sslRequired"] = r.SSLRequired
	}
	return m
}

// RealmExists reports whether realm exists (GET → 200/404).
func (c *RESTClient) RealmExists(ctx context.Context, realm string) (bool, error) {
	resp, err := c.do(ctx, http.MethodGet, "/admin/realms/"+url.PathEscape(realm), nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, apiErr("get realm", resp)
	}
}

func (c *RESTClient) CreateRealm(ctx context.Context, r Realm) error {
	resp, err := c.do(ctx, http.MethodPost, "/admin/realms", realmRep(r))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return apiErr("create realm", resp)
	}
	return nil
}

func (c *RESTClient) UpdateRealm(ctx context.Context, r Realm) error {
	resp, err := c.do(ctx, http.MethodPut, "/admin/realms/"+url.PathEscape(r.Name), realmRep(r))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return apiErr("update realm", resp)
	}
	return nil
}

func (c *RESTClient) SetRealmEnabled(ctx context.Context, realm string, enabled bool) error {
	resp, err := c.do(ctx, http.MethodPut, "/admin/realms/"+url.PathEscape(realm), map[string]any{"realm": realm, "enabled": enabled})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return apiErr("set realm enabled", resp)
	}
	return nil
}

func (c *RESTClient) DeleteRealm(ctx context.Context, realm string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/admin/realms/"+url.PathEscape(realm), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return apiErr("delete realm", resp)
	}
	return nil
}

func clientRep(cl Client) map[string]any {
	m := map[string]any{
		"clientId":                  cl.ClientID,
		"name":                      cl.Name,
		"publicClient":              cl.PublicClient,
		"standardFlowEnabled":       cl.StandardFlow,
		"directAccessGrantsEnabled": cl.DirectGrants,
	}
	if len(cl.RedirectURIs) > 0 {
		m["redirectUris"] = cl.RedirectURIs
	}
	if len(cl.WebOrigins) > 0 {
		m["webOrigins"] = cl.WebOrigins
	}
	if len(cl.Attributes) > 0 {
		m["attributes"] = cl.Attributes
	}
	return m
}

// CreateClient creates a client and returns its internal id (from Location).
func (c *RESTClient) CreateClient(ctx context.Context, realm string, cl Client) (string, error) {
	resp, err := c.do(ctx, http.MethodPost, "/admin/realms/"+url.PathEscape(realm)+"/clients", clientRep(cl))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", apiErr("create client", resp)
	}
	return idFromLocation(resp), nil
}

func (c *RESTClient) AddProtocolMapper(ctx context.Context, realm, clientID string, mp ProtocolMapper) error {
	body := map[string]any{
		"name":           mp.Name,
		"protocol":       "openid-connect",
		"protocolMapper": mp.Mapper,
		"config":         mp.Config,
	}
	resp, err := c.do(ctx, http.MethodPost,
		"/admin/realms/"+url.PathEscape(realm)+"/clients/"+url.PathEscape(clientID)+"/protocol-mappers/models", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		return apiErr("add protocol mapper", resp)
	}
	return nil
}

func (c *RESTClient) CreateRealmRole(ctx context.Context, realm, role string) error {
	resp, err := c.do(ctx, http.MethodPost, "/admin/realms/"+url.PathEscape(realm)+"/roles", map[string]any{"name": role})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		return apiErr("create realm role", resp)
	}
	return nil
}

func (c *RESTClient) CreateUser(ctx context.Context, realm string, u User) (string, error) {
	body := map[string]any{
		"username":      u.Username,
		"email":         u.Email,
		"enabled":       u.Enabled,
		"emailVerified": false,
	}
	if len(u.RequiredActions) > 0 {
		body["requiredActions"] = u.RequiredActions
	}
	resp, err := c.do(ctx, http.MethodPost, "/admin/realms/"+url.PathEscape(realm)+"/users", body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", apiErr("create user", resp)
	}
	return idFromLocation(resp), nil
}

// AssignRealmRole fetches the role representation then maps it to the user.
func (c *RESTClient) AssignRealmRole(ctx context.Context, realm, userID, role string) error {
	resp, err := c.do(ctx, http.MethodGet, "/admin/realms/"+url.PathEscape(realm)+"/roles/"+url.PathEscape(role), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return apiErr("get role", resp)
	}
	var rr struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return err
	}
	body := []map[string]any{{"id": rr.ID, "name": rr.Name}}
	resp2, err := c.do(ctx, http.MethodPost,
		"/admin/realms/"+url.PathEscape(realm)+"/users/"+url.PathEscape(userID)+"/role-mappings/realm", body)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent && resp2.StatusCode != http.StatusOK {
		return apiErr("assign realm role", resp2)
	}
	return nil
}
