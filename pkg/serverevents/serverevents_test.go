package serverevents

import (
	"context"
	"testing"
)

func TestMemBus_Delivers(t *testing.T) {
	bus := NewMemBus()
	var got Event
	bus.Register(func(e Event) { got = e })

	_ = bus.Publish(context.Background(), Event{
		Action: ActionUpsert, OrgID: "acme", Slug: "x", Type: "remote_http",
	})
	if got.OrgID != "acme" || got.Slug != "x" || got.Action != ActionUpsert {
		t.Fatalf("unexpected delivered event: %+v", got)
	}
}
