//go:build linux

package capture

import "testing"

func TestListLinuxAdapters(t *testing.T) {
	ads, err := ListLinuxAdapters()
	if err != nil {
		t.Fatal(err)
	}
	if len(ads) == 0 {
		t.Fatal("expected at least one adapter on Linux")
	}
	names := SelectIfaces(ads, nil)
	// may be empty only if nothing is up; still should not panic
	_ = names
	pinned := SelectIfaces(ads, []string{"lo"})
	if len(pinned) != 1 || pinned[0] != "lo" {
		t.Fatalf("pin lo: got %v", pinned)
	}
}
