package tenants

import (
	"fmt"
	"regexp"
	"strings"
)

// slugRe accepts a 3–40 char DNS label: starts with a letter, then lowercase
// letters/digits/hyphens, ending in a letter or digit. Also a valid Keycloak
// realm name and subdomain label.
var slugRe = regexp.MustCompile(`^[a-z][a-z0-9-]{1,38}[a-z0-9]$`)

// ValidateSlug checks a tenant slug for format and that it is not reserved.
// reservedCSV is the comma-separated reserved list (config.TenantReservedSlugs).
// Validation runs before any identity asset is created (FR-008).
func ValidateSlug(slug, reservedCSV string) error {
	if !slugRe.MatchString(slug) {
		return fmt.Errorf("invalid slug %q: 3-40 chars, lowercase letters/digits/hyphen, must start with a letter and end alphanumerically", slug)
	}
	for _, r := range strings.Split(reservedCSV, ",") {
		if r = strings.TrimSpace(r); r != "" && strings.EqualFold(r, slug) {
			return fmt.Errorf("slug %q is reserved", slug)
		}
	}
	return nil
}
