package selector

import "regexp"

// VmSku is a normalized Azure VM SKU with capability fields.
type VmSku struct {
	Name                  string   `json:"name"`
	Family                string   `json:"family"`
	VCPUs                 int      `json:"vcpus"`
	MemoryGiB             float64  `json:"memoryGiB"`
	GPUs                  int      `json:"gpus"`
	GPUMemoryGiB          float64  `json:"gpuMemoryGiB"`
	CPUArch               string   `json:"cpuArch"` // "x64" or "Arm64"
	AcceleratedNetworking bool     `json:"acceleratedNetworking"`
	PremiumIO             bool     `json:"premiumIO"`
	LocalTempDiskGiB      float64  `json:"localTempDiskGiB"`
	SpotCapable           bool     `json:"spotCapable"` // LowPriorityCapable
	ConfidentialType      string   `json:"confidentialType,omitempty"`
	Regions               []string `json:"regions"`
	MaxNICs               int      `json:"maxNICs"`
	HyperVGens            []string `json:"hyperVGens,omitempty"`
}

// Prices holds hourly on-demand and spot prices for a SKU.
type Prices struct {
	OnDemandHr float64 `json:"onDemandHr"`
	SpotHr     float64 `json:"spotHr"`
}

// Range is an optional min/max filter; nil bounds are unset.
type Range[T int | float64] struct {
	Min *T
	Max *T
}

// Filters holds all user-supplied filter criteria.
type Filters struct {
	VCPUs                 *Range[int]
	MemoryGiB             *Range[float64]
	GPUs                  *Range[int]
	CPUArch               *string
	SpotCapable           *bool
	AcceleratedNetworking *bool
	PremiumIO             *bool
	LocalDiskGiB          *Range[float64]
	Family                *regexp.Regexp
	AllowList             *regexp.Regexp
	DenyList              *regexp.Regexp
	PricePerHour          *Range[float64]
	UsageClass            string // "spot" or "on-demand"
}
