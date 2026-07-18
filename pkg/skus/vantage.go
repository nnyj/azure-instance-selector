package skus

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/nnyj/azure-instance-selector/pkg/selector"
)

// vantage instances.json URL — full file includes vcpu, memory, GPU, arch, regions.
// instances-specs.json (1MB) omits those fields; the full file (40MB) is required.
const vantageURL = "https://instances.vantage.sh/azure/instances.json"

// FetchFromVantage downloads and normalizes SKUs from Vantage's public instances.json.
// No authentication required. Progress is printed to stderr.
func FetchFromVantage(region string, verbose bool) ([]selector.VmSku, error) {
	if verbose {
		fmt.Fprintf(os.Stderr, "fetching SKUs from Vantage (%s)...\n", vantageURL)
	} else {
		fmt.Fprintln(os.Stderr, "fetching SKU catalog from Vantage (anonymous)...")
	}

	resp, err := http.Get(vantageURL)
	if err != nil {
		return nil, fmt.Errorf("vantage fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vantage HTTP %d", resp.StatusCode)
	}

	return parseVantageStream(resp.Body, region)
}

func parseVantageStream(r io.Reader, region string) ([]selector.VmSku, error) {
	var entries []vantageEntry
	if err := json.NewDecoder(r).Decode(&entries); err != nil {
		return nil, fmt.Errorf("vantage parse: %w", err)
	}
	return normalizeVantage(entries, region), nil
}

// vantageEntry matches the shape of each item in Vantage's instances.json.
// Pricing is ignored; we use Azure Retail Prices API for that.
type vantageEntry struct {
	PrettyName            string            `json:"pretty_name"`
	Family                string            `json:"family"`
	VCPUs                 int               `json:"vcpu"`
	Memory                float64           `json:"memory"` // GB, treated as GiB
	Size                  float64           `json:"size"`   // temp disk GB (top-level, 0 if none)
	GPU                   string            `json:"GPU"`
	Regions               map[string]string `json:"regions"`            // slug → display name
	HyperVGenerations     *string           `json:"hyperv_generations"` // "V1,V2" or null
	Arch                  []string          `json:"arch"`
	LowPriority           bool              `json:"low_priority"`
	PremiumIO             bool              `json:"premium_io"`
	AcceleratedNetworking bool              `json:"accelerated_networking"`
	ConfidentialType      string            `json:"confidential_type,omitempty"`
}

func normalizeVantage(entries []vantageEntry, region string) []selector.VmSku {
	armTarget := strings.ToLower(strings.ReplaceAll(region, " ", ""))
	out := make([]selector.VmSku, 0, len(entries))
	for _, e := range entries {
		if e.PrettyName == "" {
			continue
		}
		// filter by region if requested — check display names normalize to armTarget
		if region != "" && !hasRegion(e.Regions, armTarget) {
			continue
		}
		armName := vantageToARMName(e.PrettyName)
		regions := regionsToARM(e.Regions)
		sku := selector.VmSku{
			Name:                  armName,
			Family:                e.Family,
			VCPUs:                 e.VCPUs,
			MemoryGiB:             e.Memory,
			GPUs:                  parseGPUCount(e.GPU),
			CPUArch:               firstArch(e.Arch),
			AcceleratedNetworking: e.AcceleratedNetworking,
			PremiumIO:             e.PremiumIO,
			LocalTempDiskGiB:      e.Size,
			SpotCapable:           e.LowPriority,
			ConfidentialType:      e.ConfidentialType,
			Regions:               regions,
			HyperVGens:            splitHyperV(e.HyperVGenerations),
		}
		out = append(out, sku)
	}
	return out
}

// vantageToARMName converts a Vantage pretty_name to the ARM SKU name.
// e.g. "D4s v5" → "Standard_D4s_v5", "A0" → "Standard_A0"
func vantageToARMName(prettyName string) string {
	return "Standard_" + strings.ReplaceAll(prettyName, " ", "_")
}

// hasRegion returns true if any display name in regions normalizes to armTarget.
func hasRegion(regions map[string]string, armTarget string) bool {
	for _, displayName := range regions {
		norm := strings.ToLower(strings.ReplaceAll(displayName, " ", ""))
		if norm == armTarget {
			return true
		}
	}
	return false
}

// regionsToARM converts Vantage region display names to ARM region names.
func regionsToARM(regions map[string]string) []string {
	out := make([]string, 0, len(regions))
	for _, displayName := range regions {
		out = append(out, strings.ToLower(strings.ReplaceAll(displayName, " ", "")))
	}
	return out
}

// parseGPUCount extracts the GPU count from strings like "0", "1", "2X K80", "4X K80".
func parseGPUCount(s string) int {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0
	}
	// Try direct int parse first
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	// Format: "NX Type" — take the number before X
	parts := strings.SplitN(s, "X", 2)
	if len(parts) == 2 {
		n, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err == nil {
			return n
		}
	}
	return 0
}

func firstArch(arch []string) string {
	if len(arch) == 0 {
		return "x64" // legacy sizes default to x64
	}
	switch strings.ToLower(arch[0]) {
	case "arm64":
		return "Arm64"
	default:
		return "x64"
	}
}

func splitHyperV(s *string) []string {
	if s == nil || *s == "" {
		return nil
	}
	parts := strings.Split(*s, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}
