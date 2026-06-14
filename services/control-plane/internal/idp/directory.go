package idp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// Directory is the subset of the Keycloak Admin API the SCIM bridge uses to apply
// directory-sync operations (US4). Kept separate from Keycloak so provisioning
// fakes need not implement it; *RESTClient satisfies both.
type Directory interface {
	FindUserByUsername(ctx context.Context, realm, username string) (id string, found bool, err error)
	CreateUser(ctx context.Context, realm string, u User) (string, error)
	SetUserEnabled(ctx context.Context, realm, userID string, enabled bool) error
	AssignRealmRole(ctx context.Context, realm, userID, role string) error
	RemoveRealmRole(ctx context.Context, realm, userID, role string) error
}

var _ Directory = (*RESTClient)(nil)

// FindUserByUsername returns the user id for an exact username match, if present.
func (c *RESTClient) FindUserByUsername(ctx context.Context, realm, username string) (string, bool, error) {
	resp, err := c.do(ctx, http.MethodGet,
		"/admin/realms/"+url.PathEscape(realm)+"/users?exact=true&username="+url.QueryEscape(username), nil)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, apiErr("find user", resp)
	}
	var users []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return "", false, err
	}
	if len(users) == 0 {
		return "", false, nil
	}
	return users[0].ID, true, nil
}

// SetUserEnabled enables/disables a user (active=false ⇒ gateway access removed by
// the user's next token — SC-005).
func (c *RESTClient) SetUserEnabled(ctx context.Context, realm, userID string, enabled bool) error {
	resp, err := c.do(ctx, http.MethodPut,
		"/admin/realms/"+url.PathEscape(realm)+"/users/"+url.PathEscape(userID), map[string]any{"enabled": enabled})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return apiErr("set user enabled", resp)
	}
	return nil
}

// RemoveRealmRole unassigns a realm role from a user.
func (c *RESTClient) RemoveRealmRole(ctx context.Context, realm, userID, role string) error {
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
	resp2, err := c.do(ctx, http.MethodDelete,
		"/admin/realms/"+url.PathEscape(realm)+"/users/"+url.PathEscape(userID)+"/role-mappings/realm", body)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent && resp2.StatusCode != http.StatusOK {
		return apiErr("remove realm role", resp2)
	}
	return nil
}
