package config

import "testing"

func TestGetIsSingleton(t *testing.T) {
	a := Get()
	b := Get()
	if a != b {
		t.Fatal("Get() must return the same singleton instance")
	}
}

func TestDefaults(t *testing.T) {
	c := load()
	if c.HTTPAddr != ":8080" {
		t.Errorf("default HTTPAddr = %q, want :8080", c.HTTPAddr)
	}
	if c.SandboxRuntime != "gvisor" {
		t.Errorf("default SandboxRuntime = %q, want gvisor", c.SandboxRuntime)
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("MCP_HTTP_ADDR", ":9999")
	if c := load(); c.HTTPAddr != ":9999" {
		t.Errorf("env override failed: got %q", c.HTTPAddr)
	}
}
