package server

import (
	"sort"
	"strings"
	"testing"
)

func TestCredHeaders(t *testing.T) {
	h := credHeaders(map[string]string{"Authorization": "Bearer x", "X-Api-Key": "k"})
	if h.Get("Authorization") != "Bearer x" || h.Get("X-Api-Key") != "k" {
		t.Fatalf("unexpected headers: %v", h)
	}
}

func TestKVEnv(t *testing.T) {
	env := kvEnv(map[string]string{"API_KEY": "k", "TOKEN": "t"})
	sort.Strings(env)
	if strings.Join(env, ",") != "API_KEY=k,TOKEN=t" {
		t.Fatalf("unexpected env: %v", env)
	}
}
