package skus

import (
	"sort"
	"testing"
)

func TestVantageToARMName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"D4s v5", "Standard_D4s_v5"},
		{"A0", "Standard_A0"},
		{"NC6s v3", "Standard_NC6s_v3"},
	}
	for _, c := range cases {
		if got := vantageToARMName(c.in); got != c.want {
			t.Errorf("vantageToARMName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseGPUCount(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"0", 0},
		{"1", 1},
		{"2X K80", 2},
		{"4X A100", 4},
		{"garbage", 0},
		{"", 0},
	}
	for _, c := range cases {
		if got := parseGPUCount(c.in); got != c.want {
			t.Errorf("parseGPUCount(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestFirstArch(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, "x64"},
		{[]string{}, "x64"},
		{[]string{"arm64"}, "Arm64"},
		{[]string{"x64"}, "x64"},
		{[]string{"unknown"}, "x64"},
	}
	for _, c := range cases {
		if got := firstArch(c.in); got != c.want {
			t.Errorf("firstArch(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeVantage_RegionFilterAndARMNames(t *testing.T) {
	entries := []vantageEntry{
		{
			PrettyName: "D4s v5",
			VCPUs:      4,
			Memory:     16,
			Regions:    map[string]string{"eastus": "East US", "westus": "West US"},
		},
		{
			PrettyName: "A0",
			VCPUs:      1,
			Memory:     0.75,
			Regions:    map[string]string{"westus": "West US"},
		},
		{
			PrettyName: "", // empty name — must be skipped
			VCPUs:      2,
			Regions:    map[string]string{"eastus": "East US"},
		},
	}

	// region filter: only East US maps to "eastus"
	got := normalizeVantage(entries, "East US")
	if len(got) != 1 {
		t.Fatalf("region filter: got %d entries, want 1", len(got))
	}
	if got[0].Name != "Standard_D4s_v5" {
		t.Errorf("name = %q, want Standard_D4s_v5", got[0].Name)
	}

	// regionsToARM: display names → lowercase no-space slugs
	regions := append([]string(nil), got[0].Regions...)
	sort.Strings(regions)
	want := []string{"eastus", "westus"}
	if len(regions) != len(want) {
		t.Errorf("regions = %v, want %v", regions, want)
	} else {
		for i := range want {
			if regions[i] != want[i] {
				t.Errorf("regions[%d] = %q, want %q", i, regions[i], want[i])
			}
		}
	}

	// no region filter: both named entries included
	all := normalizeVantage(entries, "")
	if len(all) != 2 {
		t.Errorf("no filter: got %d entries, want 2", len(all))
	}
}
