package secrets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// VaultStore stores secrets in HashiCorp Vault's KV v2 engine at
// <mount>/data/<ref>. The token must grant create/read/delete on that path.
type VaultStore struct {
	addr  string
	token string
	mount string
	http  *http.Client
}

// NewVaultStore connects to Vault at addr using token (KV v2 mount "secret").
func NewVaultStore(addr, token string) *VaultStore {
	return &VaultStore{addr: addr, token: token, mount: "secret", http: &http.Client{Timeout: 5 * time.Second}}
}

func (v *VaultStore) dataURL(ref string) string {
	return fmt.Sprintf("%s/v1/%s/data/%s", v.addr, v.mount, ref)
}

func (v *VaultStore) do(ctx context.Context, method, url string, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Vault-Token", v.token)
	return v.http.Do(req)
}

// Put writes values as a new KV v2 version under ref.
func (v *VaultStore) Put(ctx context.Context, ref string, values map[string]string) error {
	resp, err := v.do(ctx, http.MethodPost, v.dataURL(ref), map[string]any{"data": values})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("secrets: vault put %s: status %d", ref, resp.StatusCode)
	}
	return nil
}

// Get reads the latest version at ref.
func (v *VaultStore) Get(ctx context.Context, ref string) (map[string]string, error) {
	resp, err := v.do(ctx, http.MethodGet, v.dataURL(ref), nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("secrets: vault get %s: status %d", ref, resp.StatusCode)
	}
	var out struct {
		Data struct {
			Data map[string]string `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Data.Data, nil
}

// Delete removes all versions/metadata at ref.
func (v *VaultStore) Delete(ctx context.Context, ref string) error {
	url := fmt.Sprintf("%s/v1/%s/metadata/%s", v.addr, v.mount, ref)
	resp, err := v.do(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("secrets: vault delete %s: status %d", ref, resp.StatusCode)
	}
	return nil
}
