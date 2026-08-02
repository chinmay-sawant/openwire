package wsl

import (
	"testing"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

func TestMergeHostStatsIntoAdapters(t *testing.T) {
	adapters := []domain.Adapter{
		{Name: "Wi-Fi", Source: domain.AdapterSourceWindowsHost},
		{Name: "eth0", Source: domain.AdapterSourceLinux},
	}
	prev := map[string]HostAdapterStats{
		"Wi-Fi": {Name: "Wi-Fi", RxBytes: 1000, TxBytes: 500},
	}
	stats := []HostAdapterStats{
		{Name: "Wi-Fi", RxBytes: 3000, TxBytes: 1500, Up: true},
	}
	out, next := MergeHostStatsIntoAdapters(adapters, stats, prev, 2.0)
	if out[0].RxBytes != 3000 || out[0].TxBytes != 1500 {
		t.Fatalf("counters: %+v", out[0])
	}
	if out[0].RxRateBps != 1000 || out[0].TxRateBps != 500 {
		t.Fatalf("rates: rx=%v tx=%v", out[0].RxRateBps, out[0].TxRateBps)
	}
	if out[1].Source != domain.AdapterSourceLinux {
		t.Fatal("linux adapter should be untouched")
	}
	if next["Wi-Fi"].RxBytes != 3000 {
		t.Fatalf("next prev: %+v", next)
	}
}

func TestHostTrafficObservations(t *testing.T) {
	prev := map[string]HostAdapterStats{
		"Eth": {Name: "Eth", RxBytes: 100, TxBytes: 50},
	}
	stats := []HostAdapterStats{
		{Name: "Eth", RxBytes: 1100, TxBytes: 250},
	}
	obs, next := HostTrafficObservations(time.Now(), stats, prev)
	if len(obs) != 2 {
		t.Fatalf("want 2 obs, got %d", len(obs))
	}
	if obs[0].AppHint != "windows-host" {
		t.Fatalf("app: %s", obs[0].AppHint)
	}
	if next["Eth"].RxBytes != 1100 {
		t.Fatalf("next: %+v", next)
	}
}
