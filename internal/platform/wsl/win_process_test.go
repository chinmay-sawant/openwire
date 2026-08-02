package wsl

import (
	"testing"
	"time"
)

func TestParseWindowsProcessRows(t *testing.T) {
	raw := `
1234|chrome|192.168.1.10|50000|1.1.1.1|443|tcp
1234|chrome|192.168.1.10|50001|8.8.8.8|53|udp
5678|Code|10.0.0.2|60000|93.184.216.34|443|tcp
`
	procs := parseWindowsProcessRows(raw)
	if len(procs) != 2 {
		t.Fatalf("want 2 processes, got %d: %+v", len(procs), procs)
	}
	byPID := map[int]WinProcess{}
	for _, p := range procs {
		byPID[p.PID] = p
	}
	if byPID[1234].ConnCount != 2 || byPID[1234].Name != "chrome" {
		t.Fatalf("chrome: %+v", byPID[1234])
	}
	if byPID[5678].ConnCount != 1 || byPID[5678].Name != "Code" {
		t.Fatalf("code: %+v", byPID[5678])
	}
}

func TestHostTrafficObservationsByProcessWeights(t *testing.T) {
	prev := map[string]HostAdapterStats{
		"Wi-Fi": {Name: "Wi-Fi", RxBytes: 1000, TxBytes: 100},
	}
	stats := []HostAdapterStats{
		{Name: "Wi-Fi", RxBytes: 1000 + 3000, TxBytes: 100 + 300, Up: true},
	}
	procs := []WinProcess{
		{PID: 1, Name: "chrome", ConnCount: 2, Protocol: "tcp"},
		{PID: 2, Name: "Code", ConnCount: 1, Protocol: "tcp"},
	}
	now := time.Now()
	obs, next := HostTrafficObservationsByProcess(now, stats, prev, procs)
	if next["Wi-Fi"].RxBytes != 4000 {
		t.Fatalf("next: %+v", next)
	}
	rxByApp := map[string]int{}
	txByApp := map[string]int{}
	for _, o := range obs {
		if o.Direction == "rx" {
			rxByApp[o.AppHint] += o.Length
		} else {
			txByApp[o.AppHint] += o.Length
		}
		if o.Iface != "win:Wi-Fi" {
			t.Fatalf("iface %s", o.Iface)
		}
	}
	if rxByApp["win/chrome"] != 2000 {
		t.Fatalf("chrome rx=%d want 2000 (obs=%+v)", rxByApp["win/chrome"], obs)
	}
	if rxByApp["win/Code"] != 1000 {
		t.Fatalf("code rx=%d want 1000", rxByApp["win/Code"])
	}
	if txByApp["win/chrome"] != 200 {
		t.Fatalf("chrome tx=%d want 200", txByApp["win/chrome"])
	}
	if txByApp["win/Code"] != 100 {
		t.Fatalf("code tx=%d want 100", txByApp["win/Code"])
	}
}

func TestHostTrafficObservationsByProcessFallback(t *testing.T) {
	prev := map[string]HostAdapterStats{
		"Eth": {Name: "Eth", RxBytes: 0, TxBytes: 0},
	}
	stats := []HostAdapterStats{
		{Name: "Eth", RxBytes: 500, TxBytes: 0, Up: true},
	}
	obs, _ := HostTrafficObservationsByProcess(time.Now(), stats, prev, nil)
	if len(obs) != 1 || obs[0].AppHint != "windows-host" {
		t.Fatalf("fallback: %+v", obs)
	}
	if obs[0].Iface != "win:Eth" {
		t.Fatalf("iface %s", obs[0].Iface)
	}
}
