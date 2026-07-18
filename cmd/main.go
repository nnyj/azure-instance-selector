package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/nnyj/azure-instance-selector/pkg/output"
	"github.com/nnyj/azure-instance-selector/pkg/pricing"
	"github.com/nnyj/azure-instance-selector/pkg/selector"
	"github.com/nnyj/azure-instance-selector/pkg/skus"
	flag "github.com/spf13/pflag"
)

const version = "0.1.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	fs := flag.NewFlagSet("azure-instance-selector", flag.ContinueOnError)

	// vCPU flags
	vcpus := fs.Int("vcpus", 0, "exact vCPU count (0 = unset)")
	vcpusMin := fs.Int("vcpus-min", 0, "min vCPUs (0 = unset)")
	vcpusMax := fs.Int("vcpus-max", 0, "max vCPUs (0 = unset)")

	// memory flags (accept suffixed strings like 8gb, 512mb)
	memory := fs.String("memory", "", "exact memory (e.g. 16gb, 16gib)")
	memoryMin := fs.String("memory-min", "", "min memory")
	memoryMax := fs.String("memory-max", "", "max memory")

	// GPU flags
	gpus := fs.Int("gpus", 0, "exact GPU count (0 = unset)")
	gpusMin := fs.Int("gpus-min", 0, "min GPUs (0 = unset)")
	gpusMax := fs.Int("gpus-max", 0, "max GPUs (0 = unset)")

	// price flags
	pricePerHour := fs.Float64("price-per-hour", 0, "exact price (0 = unset)")
	priceMin := fs.Float64("price-per-hour-min", 0, "min price/hr (0 = unset)")
	priceMax := fs.Float64("price-per-hour-max", 0, "max price/hr (0 = unset)")

	// string/bool filters
	cpuArch := fs.StringP("cpu-architecture", "a", "", "CPU arch: x64|arm64|x86_64|amd64")
	usageClass := fs.String("usage-class", "on-demand", "spot or on-demand")
	osFlag := fs.String("os", "linux", "linux or windows")
	spotCapable := fs.Bool("spot-capable", false, "require spot-capable SKUs")
	accelNet := fs.Bool("accelerated-networking", false, "require accelerated networking")
	premiumIO := fs.Bool("premium-io", false, "require premium IO")
	allowList := fs.String("allow-list", "", "allow-list regex on VM name")
	denyList := fs.String("deny-list", "", "deny-list regex on VM name")
	family := fs.String("family", "", "family regex")

	// output/behavior flags
	region := fs.StringP("region", "r", "eastus", "Azure region")
	maxResults := fs.Int("max-results", 20, "max results to show (0 = all)")
	outputMode := fs.StringP("output", "o", "table", "table|table-wide|one-line|json")
	skuSource := fs.String("sku-source", "vantage", "SKU data source: vantage (anonymous) or arm (requires --subscription-id)")
	subscriptionID := fs.String("subscription-id", os.Getenv("AZURE_SUBSCRIPTION_ID"), "Azure subscription ID (for --sku-source arm)")
	cacheDir := fs.String("cache-dir", defaultCacheDir(), "cache directory")
	cacheTTL := fs.Duration("cache-ttl", 168*time.Hour, "cache TTL (default 7 days)")
	refresh := fs.Bool("refresh", false, "force refresh of all caches")
	verbose := fs.BoolP("verbose", "v", false, "verbose output")
	showVersion := fs.Bool("version", false, "print version and exit")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if *showVersion {
		fmt.Println("azure-instance-selector", version)
		return nil
	}

	// build Filters
	f := selector.Filters{
		UsageClass: *usageClass,
	}

	vcpuRange := buildRange(*vcpus, *vcpusMin, *vcpusMax)
	if vcpuRange != nil {
		f.VCPUs = vcpuRange
	}

	memRange, err := buildFloat64RangeStr(*memory, *memoryMin, *memoryMax)
	if err != nil {
		return fmt.Errorf("memory: %w", err)
	}
	f.MemoryGiB = memRange

	gpuRange := buildRange(*gpus, *gpusMin, *gpusMax)
	if gpuRange != nil {
		f.GPUs = gpuRange
	}

	priceRange := buildRange(*pricePerHour, *priceMin, *priceMax)
	f.PricePerHour = priceRange

	if *cpuArch != "" {
		f.CPUArch = cpuArch
	}
	f.SpotCapable = trueIf(*spotCapable)
	f.AcceleratedNetworking = trueIf(*accelNet)
	f.PremiumIO = trueIf(*premiumIO)

	if f.AllowList, err = compileOpt("allow-list", *allowList); err != nil {
		return err
	}
	if f.DenyList, err = compileOpt("deny-list", *denyList); err != nil {
		return err
	}
	if f.Family, err = compileOpt("family", *family); err != nil {
		return err
	}

	// load SKUs
	vmSkus, err := loadSkus(*region, *skuSource, *subscriptionID, *cacheDir, *cacheTTL, *refresh, *verbose)
	if err != nil {
		return err
	}

	// load prices (anonymous, Azure Retail Prices API)
	prices, err := pricing.Fetch(*region, *osFlag, *cacheDir, *cacheTTL, *refresh, *verbose)
	if err != nil {
		return fmt.Errorf("pricing: %w", err)
	}

	results := selector.Filter(vmSkus, prices, f)

	if *maxResults > 0 && len(results) > *maxResults {
		fmt.Fprintf(os.Stderr, "%d results, showing first %d (use --max-results to change)\n", len(results), *maxResults)
		results = results[:*maxResults]
	}

	return output.Render(os.Stdout, results, prices, output.Mode(*outputMode))
}

func loadSkus(region, skuSource, subscriptionID, cacheDir string, ttl time.Duration, refresh, verbose bool) ([]selector.VmSku, error) {
	// explicit file override bypasses all other sources
	if envFile := os.Getenv("AZURE_INSTANCE_SELECTOR_SKUS_FILE"); envFile != "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "loading SKUs from %s\n", envFile)
		}
		return skus.LoadFile(envFile)
	}

	// try fresh cache (used by both sources; keyed by region)
	if !refresh {
		cached, err := skus.LoadCached(cacheDir, region, ttl)
		if err != nil {
			return nil, err
		}
		if cached != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "using cached SKUs (%d entries)\n", len(cached))
			}
			return cached, nil
		}
	}

	// fetch from chosen source
	switch skuSource {
	case "arm":
		return fetchFromARM(region, subscriptionID, cacheDir, verbose)
	default: // "vantage"
		return fetchFromVantage(region, cacheDir, verbose)
	}
}

func fetchFromVantage(region, cacheDir string, verbose bool) ([]selector.VmSku, error) {
	fetched, err := skus.FetchFromVantage(region, verbose)
	if err != nil {
		// fall back to stale cache
		fmt.Fprintf(os.Stderr, "warn: Vantage fetch failed (%v); trying stale cache\n", err)
		stale, staleErr := skus.LoadStale(cacheDir, region)
		if staleErr != nil {
			return nil, fmt.Errorf("no SKU data: Vantage unavailable and %w\n\nAlternatives:\n  --sku-source arm --subscription-id <id>\n  AZURE_INSTANCE_SELECTOR_SKUS_FILE=<path>", staleErr)
		}
		fmt.Fprintln(os.Stderr, "warn: using stale SKU cache")
		return stale, nil
	}
	if saveErr := skus.SaveCache(cacheDir, region, fetched); saveErr != nil && verbose {
		fmt.Fprintf(os.Stderr, "warn: save SKU cache: %v\n", saveErr)
	}
	return fetched, nil
}

func fetchFromARM(region, subscriptionID, cacheDir string, verbose bool) ([]selector.VmSku, error) {
	subID, err := skus.ResolveSubscriptionID(subscriptionID)
	if err != nil {
		// stale cache fallback when no subscription available
		stale, staleErr := skus.LoadStale(cacheDir, region)
		if staleErr != nil {
			return nil, fmt.Errorf("--sku-source arm requires a subscription: %w\n\nAlternatives:\n  --sku-source vantage (no auth)\n  AZURE_INSTANCE_SELECTOR_SKUS_FILE=<path>", err)
		}
		fmt.Fprintln(os.Stderr, "warn: using stale SKU cache (no subscription ID available)")
		return stale, nil
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "fetching SKUs from ARM (subscription %s, region %s)...\n", subID, region)
	}
	fetched, fetchErr := skus.FetchFromARM(context.Background(), subID, region, verbose)
	if fetchErr != nil {
		stale, staleErr := skus.LoadStale(cacheDir, region)
		if staleErr != nil {
			return nil, fmt.Errorf("ARM fetch failed: %w", fetchErr)
		}
		fmt.Fprintf(os.Stderr, "warn: ARM fetch failed (%v); using stale cache\n", fetchErr)
		return stale, nil
	}
	if saveErr := skus.SaveCache(cacheDir, region, fetched); saveErr != nil && verbose {
		fmt.Fprintf(os.Stderr, "warn: save SKU cache: %v\n", saveErr)
	}
	return fetched, nil
}

func defaultCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".azure-instance-selector"
	}
	return filepath.Join(home, ".azure-instance-selector")
}

// trueIf returns a *bool set to true when the flag was given, nil otherwise (nil = filter unset).
func trueIf(set bool) *bool {
	if !set {
		return nil
	}
	b := true
	return &b
}

// compileOpt compiles an optional regex flag; empty string means unset.
func compileOpt(name, expr string) (*regexp.Regexp, error) {
	if expr == "" {
		return nil, nil
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("%s regex: %w", name, err)
	}
	return re, nil
}

// buildRange builds an optional min/max filter from exact/min/max flags (0 = unset).
func buildRange[T int | float64](exact, min_, max_ T) *selector.Range[T] {
	if exact != 0 {
		return &selector.Range[T]{Min: &exact, Max: &exact}
	}
	if min_ == 0 && max_ == 0 {
		return nil
	}
	r := &selector.Range[T]{}
	if min_ != 0 {
		v := min_
		r.Min = &v
	}
	if max_ != 0 {
		v := max_
		r.Max = &v
	}
	return r
}

func buildFloat64RangeStr(exact, min_, max_ string) (*selector.Range[float64], error) {
	parseOpt := func(s string) (*float64, error) {
		if s == "" {
			return nil, nil
		}
		v, err := selector.ParseGiB(s)
		if err != nil {
			return nil, err
		}
		return &v, nil
	}

	if exact != "" {
		v, err := selector.ParseGiB(exact)
		if err != nil {
			return nil, err
		}
		return &selector.Range[float64]{Min: &v, Max: &v}, nil
	}
	minV, err := parseOpt(min_)
	if err != nil {
		return nil, fmt.Errorf("min: %w", err)
	}
	maxV, err := parseOpt(max_)
	if err != nil {
		return nil, fmt.Errorf("max: %w", err)
	}
	if minV == nil && maxV == nil {
		return nil, nil
	}
	return &selector.Range[float64]{Min: minV, Max: maxV}, nil
}
