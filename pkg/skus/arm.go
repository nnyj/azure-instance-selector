package skus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/nnyj/azure-instance-selector/pkg/selector"
)

const armAPIVersion = "2021-07-01"

// FetchFromARM retrieves VM SKUs from Azure Resource Manager for the given region and subscription.
func FetchFromARM(ctx context.Context, subscriptionID, region string, verbose bool) ([]selector.VmSku, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("credential: %w", err)
	}

	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://management.azure.com/.default"},
	})
	if err != nil {
		return nil, fmt.Errorf("get token: %w", err)
	}

	filter := url.QueryEscape(fmt.Sprintf("location eq '%s'", region))
	next := fmt.Sprintf(
		"https://management.azure.com/subscriptions/%s/providers/Microsoft.Compute/skus?api-version=%s&$filter=%s",
		subscriptionID, armAPIVersion, filter,
	)

	var all []armSku
	for next != "" {
		if verbose {
			fmt.Fprintf(os.Stderr, "ARM SKU fetch: %s\n", next)
		}
		page, err := getARMPage(ctx, next, tok.Token)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Value...)
		next = page.NextLink
	}

	return normalizeSkus(all, region), nil
}

func getARMPage(ctx context.Context, u, token string) (*armSkuPage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ARM request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("ARM API %d: %s – %s", resp.StatusCode, errBody.Error.Code, errBody.Error.Message)
	}

	var page armSkuPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("decode ARM response: %w", err)
	}
	return &page, nil
}

// ResolveSubscriptionID returns subscriptionID from env or `az account show`.
func ResolveSubscriptionID(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	out, err := exec.Command("az", "account", "show", "--query", "[id,name]", "-o", "tsv").Output()
	if err != nil {
		return "", fmt.Errorf("no subscription ID: set --subscription-id or AZURE_SUBSCRIPTION_ID, or run `az login`")
	}
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	id := strings.TrimSpace(lines[0])
	name := ""
	if len(lines) > 1 {
		name = strings.TrimSpace(lines[1])
	}
	// tenant-level accounts (az login --allow-no-subscriptions) report the tenant ID as account id
	if id == "" || strings.HasPrefix(name, "N/A") {
		return "", fmt.Errorf("signed-in account has no subscription (tenant-level only); create one or pass --subscription-id")
	}
	return id, nil
}

// arm API types

type armSkuPage struct {
	Value    []armSku `json:"value"`
	NextLink string   `json:"nextLink"`
}

type armSku struct {
	Name         string           `json:"name"`
	Family       string           `json:"family"`
	ResourceType string           `json:"resourceType"`
	Locations    []string         `json:"locations"`
	Capabilities []armCapability  `json:"capabilities"`
	Restrictions []armRestriction `json:"restrictions"`
}

type armCapability struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type armRestriction struct {
	Type   string `json:"type"`
	Reason string `json:"reasonCode"`
}

func normalizeSkus(raw []armSku, region string) []selector.VmSku {
	out := make([]selector.VmSku, 0, len(raw))
	regionLower := strings.ToLower(region)
	for _, r := range raw {
		if !strings.EqualFold(r.ResourceType, "virtualMachines") {
			continue
		}
		// skip if restricted (NotAvailableForSubscription)
		if isFullyRestricted(r.Restrictions) {
			continue
		}
		caps := capMap(r.Capabilities)
		sku := selector.VmSku{
			Name:                  r.Name,
			Family:                r.Family,
			VCPUs:                 parseInt(caps["vCPUs"]),
			MemoryGiB:             parseFloat(caps["MemoryGB"]),
			GPUs:                  parseInt(caps["GPUs"]),
			CPUArch:               normArmArch(caps["CpuArchitectureType"]),
			AcceleratedNetworking: parseBool(caps["AcceleratedNetworkingEnabled"]),
			PremiumIO:             parseBool(caps["PremiumIO"]),
			// MaxResourceVolumeMB is temp disk; convert to GiB
			LocalTempDiskGiB: parseFloat(caps["MaxResourceVolumeMB"]) / 1024,
			SpotCapable:      parseBool(caps["LowPriorityCapable"]),
			ConfidentialType: caps["ConfidentialComputingType"],
			MaxNICs:          parseInt(caps["MaxNetworkInterfaces"]),
			HyperVGens:       splitComma(caps["HyperVGenerations"]),
			Regions:          []string{regionLower},
		}
		// GPUMemory: not always present in capabilities; best-effort
		if v, ok := caps["GPUMemoryGB"]; ok {
			sku.GPUMemoryGiB = parseFloat(v)
		}
		out = append(out, sku)
	}
	return out
}

func capMap(caps []armCapability) map[string]string {
	m := make(map[string]string, len(caps))
	for _, c := range caps {
		m[c.Name] = c.Value
	}
	return m
}

func isFullyRestricted(rs []armRestriction) bool {
	for _, r := range rs {
		if strings.EqualFold(r.Reason, "NotAvailableForSubscription") {
			return true
		}
	}
	return false
}

func parseInt(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

func parseBool(s string) bool {
	return strings.EqualFold(strings.TrimSpace(s), "true")
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

func normArmArch(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "arm64":
		return "Arm64"
	default:
		return "x64"
	}
}
