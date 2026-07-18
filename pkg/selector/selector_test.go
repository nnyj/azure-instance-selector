package selector

import (
	"regexp"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestParseGiB(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"8gb", 8, true},
		{"8gib", 8, true},
		{"512mb", 0.5, true},
		{"512mib", 0.5, true},
		{"16", 16, true},
		{"1tb", 1024, true},
		{"bad", 0, false},
	}
	for _, c := range cases {
		got, err := ParseGiB(c.in)
		if c.ok && err != nil {
			t.Errorf("ParseGiB(%q) error: %v", c.in, err)
		}
		if !c.ok && err == nil {
			t.Errorf("ParseGiB(%q) expected error", c.in)
		}
		if c.ok && got != c.want {
			t.Errorf("ParseGiB(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNormArch(t *testing.T) {
	cases := []struct{ in, want string }{
		{"x64", "x64"}, {"x86_64", "x64"}, {"amd64", "x64"},
		{"arm64", "arm64"}, {"aarch64", "arm64"},
		{"Arm64", "arm64"},
	}
	for _, c := range cases {
		got := normArch(c.in)
		if got != c.want {
			t.Errorf("normArch(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFilter_VCPUs(t *testing.T) {
	skus := []VmSku{
		{Name: "A", VCPUs: 2, MemoryGiB: 8},
		{Name: "B", VCPUs: 4, MemoryGiB: 16},
		{Name: "C", VCPUs: 8, MemoryGiB: 32},
	}
	prices := map[string]Prices{}
	got := Filter(skus, prices, Filters{VCPUs: &Range[int]{Min: ptr(4), Max: ptr(4)}})
	if len(got) != 1 || got[0].Name != "B" {
		t.Errorf("vcpu exact filter: %v", got)
	}
	got = Filter(skus, prices, Filters{VCPUs: &Range[int]{Max: ptr(4)}})
	if len(got) != 2 {
		t.Errorf("vcpu max filter: %v", got)
	}
}

func TestFilter_Memory(t *testing.T) {
	skus := []VmSku{
		{Name: "A", MemoryGiB: 8},
		{Name: "B", MemoryGiB: 16},
		{Name: "C", MemoryGiB: 32},
	}
	prices := map[string]Prices{}
	got := Filter(skus, prices, Filters{MemoryGiB: &Range[float64]{Min: ptr(16.0)}})
	if len(got) != 2 {
		t.Errorf("memory min filter: %v", got)
	}
}

func TestFilter_Arch(t *testing.T) {
	skus := []VmSku{
		{Name: "A", CPUArch: "x64"},
		{Name: "B", CPUArch: "Arm64"},
	}
	prices := map[string]Prices{}
	got := Filter(skus, prices, Filters{CPUArch: ptr("arm64")})
	if len(got) != 1 || got[0].Name != "B" {
		t.Errorf("arch filter: %v", got)
	}
	got = Filter(skus, prices, Filters{CPUArch: ptr("amd64")})
	if len(got) != 1 || got[0].Name != "A" {
		t.Errorf("arch alias filter: %v", got)
	}
}

func TestFilter_AllowDenyList(t *testing.T) {
	skus := []VmSku{
		{Name: "Standard_D4s_v5"},
		{Name: "Standard_D8s_v5"},
		{Name: "Standard_E4s_v5"},
	}
	prices := map[string]Prices{}
	got := Filter(skus, prices, Filters{AllowList: regexp.MustCompile(`_D`)})
	if len(got) != 2 {
		t.Errorf("allow-list: %v", got)
	}
	got = Filter(skus, prices, Filters{DenyList: regexp.MustCompile(`_D8`)})
	if len(got) != 2 {
		t.Errorf("deny-list: %v", got)
	}
}

func TestFilter_SpotCapable(t *testing.T) {
	skus := []VmSku{
		{Name: "A", SpotCapable: true},
		{Name: "B", SpotCapable: false},
	}
	prices := map[string]Prices{}
	got := Filter(skus, prices, Filters{SpotCapable: ptr(true)})
	if len(got) != 1 || got[0].Name != "A" {
		t.Errorf("spot-capable filter: %v", got)
	}
}

func TestFilter_PriceRange(t *testing.T) {
	skus := []VmSku{
		{Name: "A"},
		{Name: "B"},
		{Name: "C"},
	}
	prices := map[string]Prices{
		"A": {OnDemandHr: 0.1},
		"B": {OnDemandHr: 0.5},
		"C": {OnDemandHr: 1.0},
	}
	got := Filter(skus, prices, Filters{
		PricePerHour: &Range[float64]{Max: ptr(0.5)},
		UsageClass:   "on-demand",
	})
	if len(got) != 2 {
		t.Errorf("price max filter: %v", got)
	}
}

func TestFilter_SortByPrice(t *testing.T) {
	skus := []VmSku{
		{Name: "B"},
		{Name: "A"},
		{Name: "C"},
	}
	prices := map[string]Prices{
		"A": {OnDemandHr: 0.3},
		"B": {OnDemandHr: 0.1},
		"C": {OnDemandHr: 0.2},
	}
	got := Filter(skus, prices, Filters{UsageClass: "on-demand"})
	if len(got) != 3 || got[0].Name != "B" || got[1].Name != "C" || got[2].Name != "A" {
		t.Errorf("sort by price: %v", got)
	}
}
