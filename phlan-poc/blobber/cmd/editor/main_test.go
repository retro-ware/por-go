package main

import (
	"os"
	"strings"
	"testing"

	"blobber/events"
)

// The demo files must survive a load/serialize round trip and still parse.
func TestRoundTrip(t *testing.T) {
	os.Chdir("../..")
	e := &editor{name: "demo"}
	e.load()
	if len(e.evs) != 9 || len(e.grid) != 14 {
		t.Fatalf("events=%d rows=%d status=%q", len(e.evs), len(e.grid), e.status)
	}
	txt := e.evtText()
	if err := events.New(nil).Load("demo", strings.NewReader(txt)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(txt, "event 3,5 any use\n  if mapflag searched goto done") || !strings.Contains(txt, "\ndone:\n  text") {
		t.Fatalf("bad output:\n%s", txt)
	}
}
