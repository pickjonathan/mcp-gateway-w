package idp

import (
	"context"
	"encoding/json"
	"net/http"
)

// RealmLister lists realm names. Used by the dev-only org discovery endpoint so
// the console picker reflects whatever realms exist in Keycloak (including ones
// created by hand). Implemented by *RESTClient.
type RealmLister interface {
	ListRealms(ctx context.Context) ([]string, error)
}

var _ RealmLister = (*RESTClient)(nil)

// ListRealms returns all realm names (brief representation) the admin credential
// can see.
func (c *RESTClient) ListRealms(ctx context.Context) ([]string, error) {
	resp, err := c.do(ctx, http.MethodGet, "/admin/realms?briefRepresentation=true", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, apiErr("list realms", resp)
	}
	var realms []struct {
		Realm string `json:"realm"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&realms); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(realms))
	for _, r := range realms {
		if r.Realm != "" {
			out = append(out, r.Realm)
		}
	}
	return out, nil
}
