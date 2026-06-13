package aggregate

import "testing"

func TestAggregate_NamespacingNoCollision(t *testing.T) {
	in := []ServerTools{
		{Slug: "github", Tools: []Tool{{Name: "search"}, {Name: "create_issue"}}},
		{Slug: "jira", Tools: []Tool{{Name: "search"}}},
	}
	r := Aggregate(in)

	if len(r.Tools) != 3 {
		t.Fatalf("want 3 namespaced tools, got %d", len(r.Tools))
	}
	for _, want := range []string{"github__search", "github__create_issue", "jira__search"} {
		if _, ok := r.Routes[want]; !ok {
			t.Errorf("missing route %q", want)
		}
	}
	// Same tool name on two servers must remain distinct.
	if r.Routes["github__search"].Slug == r.Routes["jira__search"].Slug {
		t.Fatal("namespacing failed: cross-server collision")
	}
	if got := r.Routes["jira__search"]; got.Slug != "jira" || got.Tool != "search" {
		t.Fatalf("route resolves wrong: %+v", got)
	}
}

func TestAggregate_DuplicateWithinServerKeepsFirst(t *testing.T) {
	in := []ServerTools{{Slug: "x", Tools: []Tool{{Name: "a"}, {Name: "a"}}}}
	if r := Aggregate(in); len(r.Tools) != 1 {
		t.Fatalf("duplicate within a server should keep first; got %d tools", len(r.Tools))
	}
}
