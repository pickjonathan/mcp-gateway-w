// Package aggregate merges the capabilities of multiple downstream MCP servers
// into a single, collision-safe namespaced surface (FR-003).
package aggregate

import "encoding/json"

// Separator joins a server slug and a capability name into a namespaced name.
const Separator = "__"

// Tool is a downstream MCP tool descriptor.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// ServerTools is the set of tools exposed by one downstream server.
type ServerTools struct {
	Slug  string
	Tools []Tool
}

// Route resolves a namespaced tool name back to its server and original name.
type Route struct {
	Slug string
	Tool string
}

// Result is the aggregated, namespaced surface plus its routing table.
type Result struct {
	Tools  []Tool
	Routes map[string]Route
}

// Aggregate merges tools across servers, namespacing each as
// "<slug><Separator><tool>". Server slugs are assumed unique (enforced upstream);
// within a server, duplicate tool names keep the first occurrence. No tool is
// ever silently overwritten across servers.
func Aggregate(servers []ServerTools) Result {
	res := Result{Tools: make([]Tool, 0), Routes: make(map[string]Route)}
	for _, s := range servers {
		for _, t := range s.Tools {
			name := s.Slug + Separator + t.Name
			if _, exists := res.Routes[name]; exists {
				continue
			}
			res.Routes[name] = Route{Slug: s.Slug, Tool: t.Name}
			nt := t
			nt.Name = name
			res.Tools = append(res.Tools, nt)
		}
	}
	return res
}
