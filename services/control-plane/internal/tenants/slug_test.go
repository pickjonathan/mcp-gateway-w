package tenants

import "testing"

func TestValidateSlug(t *testing.T) {
	const reserved = "www,api,admin,auth,app"
	cases := []struct {
		slug string
		ok   bool
	}{
		{"globex", true},
		{"acme-corp", true},
		{"a1b", true},
		{"ab", false},      // too short (<3)
		{"1globex", false}, // must start with a letter
		{"Globex", false},  // uppercase not allowed
		{"globex-", false}, // trailing hyphen
		{"-globex", false}, // leading hyphen
		{"a.b", false},     // dot not a valid label char
		{"www", false},     // reserved
		{"API", false},     // reserved (case-insensitive)
		{"", false},        // empty
	}
	for _, tc := range cases {
		err := ValidateSlug(tc.slug, reserved)
		if (err == nil) != tc.ok {
			t.Errorf("ValidateSlug(%q): got err=%v, want ok=%v", tc.slug, err, tc.ok)
		}
	}
}
