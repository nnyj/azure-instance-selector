package selector

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// Filter returns skus matching all non-nil criteria in f, using prices for price filters.
// Results are sorted by on-demand price (or spot price when f.UsageClass=="spot"), then name.
func Filter(skus []VmSku, prices map[string]Prices, f Filters) []VmSku {
	var out []VmSku
	for _, s := range skus {
		if matchSku(s, prices, f) {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		pi := priceFor(out[i].Name, prices, f.UsageClass)
		pj := priceFor(out[j].Name, prices, f.UsageClass)
		if pi != pj {
			return pi < pj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func priceFor(name string, prices map[string]Prices, usageClass string) float64 {
	p := prices[name]
	hr := p.OnDemandHr
	if usageClass == "spot" {
		hr = p.SpotHr
	}
	if hr == 0 {
		// missing price sorts last, not first
		return math.MaxFloat64
	}
	return hr
}

func matchSku(s VmSku, prices map[string]Prices, f Filters) bool {
	if !matchRange(s.VCPUs, f.VCPUs) {
		return false
	}
	if !matchRange(s.MemoryGiB, f.MemoryGiB) {
		return false
	}
	if !matchRange(s.GPUs, f.GPUs) {
		return false
	}
	if f.CPUArch != nil && !strings.EqualFold(normArch(s.CPUArch), normArch(*f.CPUArch)) {
		return false
	}
	if f.SpotCapable != nil && s.SpotCapable != *f.SpotCapable {
		return false
	}
	if f.AcceleratedNetworking != nil && s.AcceleratedNetworking != *f.AcceleratedNetworking {
		return false
	}
	if f.PremiumIO != nil && s.PremiumIO != *f.PremiumIO {
		return false
	}
	if !matchRange(s.LocalTempDiskGiB, f.LocalDiskGiB) {
		return false
	}
	if f.Family != nil && !f.Family.MatchString(s.Family) {
		return false
	}
	if f.AllowList != nil && !f.AllowList.MatchString(s.Name) {
		return false
	}
	if f.DenyList != nil && f.DenyList.MatchString(s.Name) {
		return false
	}
	if f.PricePerHour != nil {
		p, ok := prices[s.Name]
		if !ok {
			return false
		}
		var hr float64
		if f.UsageClass == "spot" {
			hr = p.SpotHr
			if hr == 0 {
				return false
			}
		} else {
			hr = p.OnDemandHr
			if hr == 0 {
				return false
			}
		}
		if !matchRange(hr, f.PricePerHour) {
			return false
		}
	}
	return true
}

func matchRange[T int | float64](v T, r *Range[T]) bool {
	if r == nil {
		return true
	}
	if r.Min != nil && v < *r.Min {
		return false
	}
	if r.Max != nil && v > *r.Max {
		return false
	}
	return true
}

// normArch maps common CPU arch aliases to canonical form for comparison.
func normArch(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "x64", "x86_64", "amd64":
		return "x64"
	case "arm64", "aarch64":
		return "arm64"
	}
	return strings.ToLower(s)
}

// ParseGiB parses a memory string like "8gb", "512mb", "16gib", "4" (bare GiB).
func ParseGiB(s string) (float64, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, suffix := range []struct {
		suf string
		div float64
	}{
		{"gib", 1}, {"gb", 1}, {"mib", 1024}, {"mb", 1024},
		{"tib", 1.0 / 1024}, {"tb", 1.0 / 1024},
	} {
		if strings.HasSuffix(s, suffix.suf) {
			num := strings.TrimSuffix(s, suffix.suf)
			v, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
			if err != nil {
				return 0, err
			}
			if suffix.div == 1 {
				return v, nil
			}
			return v / suffix.div, nil
		}
	}
	// bare number treated as GiB
	return strconv.ParseFloat(s, 64)
}
