package telemetry

import (
	"bytes"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	cases := []struct {
		name           string
		in             string
		mustNotContain []string
		mustContain    []string
	}{
		{"json auth header", `{"authorization":"Bearer abc.def"}`,
			[]string{"abc.def"}, []string{Redacted, "authorization"}},
		{"json api key", `{"api_key":"shh"}`, []string{"shh"}, []string{Redacted}},
		{"prefixed header key", `{"x-api-key":"shh"}`, []string{"shh"}, []string{Redacted, "x-api-key"}},
		{"header array", `{"Authorization":["Bearer xyz"]}`, []string{"xyz"}, []string{Redacted}},
		{"env kv", `{"env":["API_KEY=secretval","HOME=/root"]}`,
			[]string{"secretval"}, []string{Redacted, "HOME=/root"}},
		{"bare bearer", `msg with Bearer tok123 inside`, []string{"tok123"}, []string{Redacted}},
		{"non-sensitive preserved", `{"user":"alice","count":5,"credential_mode":"per_user"}`,
			[]string{Redacted}, []string{"alice", "count", "per_user"}},
	}
	for _, c := range cases {
		got := Redact(c.in)
		for _, s := range c.mustNotContain {
			if strings.Contains(got, s) {
				t.Errorf("%s: %q must not contain %q", c.name, got, s)
			}
		}
		for _, s := range c.mustContain {
			if !strings.Contains(got, s) {
				t.Errorf("%s: %q must contain %q", c.name, got, s)
			}
		}
	}
}

func TestRedactingWriter(t *testing.T) {
	var buf bytes.Buffer
	w := RedactingWriter{W: &buf}
	in := `{"authorization":"Bearer s3cr3t"}` + "\n"

	n, err := w.Write([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if n != len(in) {
		t.Fatalf("Write must report the original length %d, got %d", len(in), n)
	}
	if strings.Contains(buf.String(), "s3cr3t") {
		t.Fatalf("secret leaked through writer: %s", buf.String())
	}
	if !strings.Contains(buf.String(), Redacted) {
		t.Fatalf("value not redacted: %s", buf.String())
	}
}
