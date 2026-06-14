package tenants

import (
	"context"
	"fmt"
	"sync"

	"github.com/acme-corp/mcp-runtime/services/control-plane/internal/idp"
)

// fakeKC is an in-memory idp.Keycloak for hermetic tests. It records calls and can
// be configured to fail a specific method (failOn) to exercise saga compensation.
type fakeKC struct {
	mu      sync.Mutex
	realms  map[string]bool
	enabled map[string]bool
	calls   []string
	failOn  string
}

func newFakeKC() *fakeKC {
	return &fakeKC{realms: map[string]bool{}, enabled: map[string]bool{}}
}

var _ idp.Keycloak = (*fakeKC)(nil)

func (f *fakeKC) rec(s string) {
	f.mu.Lock()
	f.calls = append(f.calls, s)
	f.mu.Unlock()
}

func (f *fakeKC) fail(m string) error {
	if f.failOn == m {
		return fmt.Errorf("injected failure in %s", m)
	}
	return nil
}

func (f *fakeKC) called(s string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if c == s {
			return true
		}
	}
	return false
}

func (f *fakeKC) RealmExists(_ context.Context, realm string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.realms[realm], nil
}

func (f *fakeKC) CreateRealm(_ context.Context, r idp.Realm) error {
	if err := f.fail("CreateRealm"); err != nil {
		return err
	}
	f.rec("CreateRealm:" + r.Name)
	f.mu.Lock()
	f.realms[r.Name] = true
	f.enabled[r.Name] = true
	f.mu.Unlock()
	return nil
}

func (f *fakeKC) UpdateRealm(_ context.Context, r idp.Realm) error { f.rec("UpdateRealm:" + r.Name); return nil }

func (f *fakeKC) SetRealmEnabled(_ context.Context, realm string, en bool) error {
	if err := f.fail("SetRealmEnabled"); err != nil {
		return err
	}
	f.rec(fmt.Sprintf("SetRealmEnabled:%s:%v", realm, en))
	f.mu.Lock()
	f.enabled[realm] = en
	f.mu.Unlock()
	return nil
}

func (f *fakeKC) DeleteRealm(_ context.Context, realm string) error {
	f.rec("DeleteRealm:" + realm)
	f.mu.Lock()
	delete(f.realms, realm)
	f.mu.Unlock()
	return nil
}

func (f *fakeKC) CreateClient(_ context.Context, realm string, c idp.Client) (string, error) {
	if err := f.fail("CreateClient"); err != nil {
		return "", err
	}
	f.rec("CreateClient:" + realm + ":" + c.ClientID)
	return "id-" + c.ClientID, nil
}

func (f *fakeKC) AddProtocolMapper(_ context.Context, realm, clientID string, m idp.ProtocolMapper) error {
	if err := f.fail("AddProtocolMapper"); err != nil {
		return err
	}
	f.rec("AddProtocolMapper:" + m.Name)
	return nil
}

func (f *fakeKC) CreateRealmRole(_ context.Context, realm, role string) error {
	if err := f.fail("CreateRealmRole"); err != nil {
		return err
	}
	f.rec("CreateRealmRole:" + role)
	return nil
}

func (f *fakeKC) CreateUser(_ context.Context, realm string, u idp.User) (string, error) {
	if err := f.fail("CreateUser"); err != nil {
		return "", err
	}
	f.rec("CreateUser:" + u.Username)
	return "uid-1", nil
}

func (f *fakeKC) AssignRealmRole(_ context.Context, realm, userID, role string) error {
	if err := f.fail("AssignRealmRole"); err != nil {
		return err
	}
	f.rec("AssignRealmRole:" + role)
	return nil
}
