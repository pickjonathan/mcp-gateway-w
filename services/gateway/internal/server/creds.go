package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/acme-corp/mcp-runtime/pkg/secrets"
	"github.com/acme-corp/mcp-runtime/pkg/serverevents"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/downstream"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/remotehttp"
	"github.com/acme-corp/mcp-runtime/services/gateway/internal/sandbox"
)

// resolveCredentials fetches the credential values to inject for a server based
// on its credential_mode. Returns nil for none/unset or when not found.
func (s *Server) resolveCredentials(e serverevents.Event) map[string]string {
	switch e.CredentialMode {
	case "", "none":
		return nil
	case "org_shared":
		v, err := s.secrets.Get(context.Background(), secrets.OrgRef(e.OrgID, e.ID))
		if err != nil {
			return nil
		}
		return v
	case "per_user":
		// Handled via a per-user Provider (perUserProvider) wired in
		// applyServerEvent, not here — this path builds a single shared instance.
		return nil
	default:
		return nil
	}
}

// perUserProvider returns a downstream.Provider that builds a per-user client for
// e, injecting that user's stored credentials (per_user mode, US6). It errors for
// users with no credentials configured, leaving the server invisible and
// uncallable for them until they provide credentials.
func (s *Server) perUserProvider(e serverevents.Event) downstream.Provider {
	return func(user string) (downstream.Downstream, error) {
		creds, err := s.secrets.Get(context.Background(), secrets.UserRef(e.OrgID, e.ID, user))
		if err != nil {
			return nil, fmt.Errorf("per-user credentials for %s/%s: %w", e.Slug, user, err)
		}
		switch e.Type {
		case "remote_http":
			return remotehttp.New(e.EndpointURL, remotehttp.WithBlockPrivate(s.blockEgress), remotehttp.WithHeader(credHeaders(creds))), nil
		case "stdio":
			env := make([]string, 0, len(e.Env)+len(creds))
			for k, v := range e.Env {
				env = append(env, k+"="+v)
			}
			env = append(env, kvEnv(creds)...)
			return sandbox.NewServer(s.runtime, sandbox.Spec{Command: e.Command, Args: e.Args, Env: env}), nil
		default:
			return nil, fmt.Errorf("per-user credentials unsupported for type %q", e.Type)
		}
	}
}

// credHeaders renders credential key/values as HTTP headers (remote servers).
func credHeaders(creds map[string]string) http.Header {
	h := http.Header{}
	for k, v := range creds {
		h.Set(k, v)
	}
	return h
}

// kvEnv renders credential key/values as KEY=VALUE env entries (stdio servers).
func kvEnv(creds map[string]string) []string {
	out := make([]string, 0, len(creds))
	for k, v := range creds {
		out = append(out, k+"="+v)
	}
	return out
}
