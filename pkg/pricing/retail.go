package pricing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nnyj/azure-instance-selector/pkg/selector"
)

const retailBaseURL = "https://prices.azure.com/api/retail/prices"

// Fetch returns on-demand and spot prices for the given region and OS.
// Results are cached to disk in cacheDir. If cache is fresh, no network call is made.
// Progress dots are printed to stderr during pagination.
func Fetch(region, os_, cacheDir string, ttl time.Duration, refresh, verbose bool) (map[string]selector.Prices, error) {
	cachefile := filepath.Join(cacheDir, fmt.Sprintf("prices_%s_%s.json", region, os_))

	if !refresh {
		if p, err := loadPriceCache(cachefile, ttl); p != nil {
			return p, nil
		} else if err != nil && verbose {
			fmt.Fprintf(os.Stderr, "price cache: %v\n", err)
		}
	}

	items, err := fetchAllPages(region, verbose)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(os.Stderr) // newline after progress dots

	prices := classifyPrices(items, os_)

	if err := savePriceCache(cachefile, prices); err != nil && verbose {
		fmt.Fprintf(os.Stderr, "warn: save price cache: %v\n", err)
	}
	return prices, nil
}

// retailItem matches the JSON structure of each entry in the Retail Prices API.
type retailItem struct {
	ArmSkuName  string  `json:"armSkuName"`
	SkuName     string  `json:"skuName"`
	ProductName string  `json:"productName"`
	MeterName   string  `json:"meterName"`
	UnitPrice   float64 `json:"unitPrice"`
	RetailPrice float64 `json:"retailPrice"`
}

type retailPage struct {
	Items        []retailItem `json:"Items"`
	NextPageLink string       `json:"NextPageLink"`
}

func fetchAllPages(region string, verbose bool) ([]retailItem, error) {
	filter := fmt.Sprintf("serviceName eq 'Virtual Machines' and armRegionName eq '%s' and priceType eq 'Consumption'", region)
	startURL := retailBaseURL + "?$filter=" + url.QueryEscape(filter)

	var all []retailItem
	next := startURL
	page := 0
	for next != "" {
		page++
		if verbose {
			fmt.Fprintf(os.Stderr, "\rfetching pricing page %d...", page)
		} else {
			fmt.Fprint(os.Stderr, ".")
		}

		var p retailPage
		if err := getJSON(next, &p); err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}
		all = append(all, p.Items...)
		next = p.NextPageLink
	}
	return all, nil
}

func getJSON(u string, out any) error {
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, u)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// classifyPrices builds a SKU→Prices map from retail items.
// os_ is "linux" or "windows". Linux prices are items whose productName does NOT contain "Windows".
// Spot prices have "Spot" in skuName or meterName. Low Priority entries are skipped.
func classifyPrices(items []retailItem, os_ string) map[string]selector.Prices {
	prices := make(map[string]selector.Prices, len(items)/2)
	for _, it := range items {
		if it.ArmSkuName == "" {
			continue
		}
		// skip low-priority (different billing model, not standard spot)
		if containsCI(it.SkuName, "low priority") || containsCI(it.MeterName, "low priority") {
			continue
		}
		isWindows := containsCI(it.ProductName, "windows")
		if os_ == "windows" && !isWindows {
			continue
		}
		if os_ != "windows" && isWindows {
			continue
		}

		isSpot := containsCI(it.SkuName, "spot") || containsCI(it.MeterName, "spot")
		p := prices[it.ArmSkuName]
		price := it.UnitPrice
		if price == 0 {
			price = it.RetailPrice
		}
		if isSpot {
			if p.SpotHr == 0 || price < p.SpotHr {
				p.SpotHr = price
			}
		} else {
			if p.OnDemandHr == 0 || price < p.OnDemandHr {
				p.OnDemandHr = price
			}
		}
		prices[it.ArmSkuName] = p
	}
	return prices
}

func containsCI(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// priceCache is the on-disk format for cached prices.
type priceCache struct {
	FetchedAt time.Time                  `json:"fetchedAt"`
	Prices    map[string]selector.Prices `json:"prices"`
}

func loadPriceCache(path string, ttl time.Duration) (map[string]selector.Prices, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var c priceCache
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return nil, err
	}
	if time.Since(c.FetchedAt) > ttl {
		return nil, nil // stale
	}
	return c.Prices, nil
}

func savePriceCache(path string, prices map[string]selector.Prices) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(priceCache{FetchedAt: time.Now(), Prices: prices})
}
