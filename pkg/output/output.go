package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/nnyj/azure-instance-selector/pkg/selector"
)

// Mode is the output format.
type Mode string

const (
	ModeTable     Mode = "table"
	ModeTableWide Mode = "table-wide"
	ModeOneLine   Mode = "one-line"
	ModeJSON      Mode = "json"
)

// vmSkuWithPrices combines a VmSku with its pricing for JSON output.
type vmSkuWithPrices struct {
	selector.VmSku
	OnDemandHr float64 `json:"onDemandHr"`
	SpotHr     float64 `json:"spotHr"`
	SpotSaving float64 `json:"spotSavingPct,omitempty"`
}

// Render writes results to w in the requested mode.
func Render(w io.Writer, skus []selector.VmSku, prices map[string]selector.Prices, mode Mode) error {
	switch mode {
	case ModeOneLine:
		return renderOneLine(w, skus)
	case ModeJSON:
		return renderJSON(w, skus, prices)
	case ModeTableWide:
		return renderTable(w, skus, prices, true)
	default:
		return renderTable(w, skus, prices, false)
	}
}

func renderOneLine(w io.Writer, skus []selector.VmSku) error {
	names := make([]string, len(skus))
	for i, s := range skus {
		names[i] = s.Name
	}
	_, err := fmt.Fprintln(w, strings.Join(names, " "))
	return err
}

func renderJSON(w io.Writer, skus []selector.VmSku, prices map[string]selector.Prices) error {
	out := make([]vmSkuWithPrices, len(skus))
	for i, s := range skus {
		p := prices[s.Name]
		saving := spotSaving(p.OnDemandHr, p.SpotHr)
		out[i] = vmSkuWithPrices{
			VmSku:      s,
			OnDemandHr: p.OnDemandHr,
			SpotHr:     p.SpotHr,
			SpotSaving: saving,
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func renderTable(w io.Writer, skus []selector.VmSku, prices map[string]selector.Prices, wide bool) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if wide {
		fmt.Fprintln(tw, "NAME\tVCPUs\tMEM GiB\tGPUs\tON-DEMAND $/hr\tSPOT $/hr\tSPOT SAVE%\tARCH\tFAMILY\tACCEL-NET\tPREMIUM-IO\tTEMP DISK GiB\tSPOT CAP")
	} else {
		fmt.Fprintln(tw, "NAME\tVCPUs\tMEM GiB\tGPUs\tON-DEMAND $/hr\tSPOT $/hr\tSPOT SAVE%")
	}
	for _, s := range skus {
		p := prices[s.Name]
		saving := spotSaving(p.OnDemandHr, p.SpotHr)
		od := fmtPrice(p.OnDemandHr)
		sp := fmtPrice(p.SpotHr)
		sav := fmtSaving(saving)
		if wide {
			fmt.Fprintf(tw, "%s\t%d\t%.1f\t%d\t%s\t%s\t%s\t%s\t%s\t%v\t%v\t%.1f\t%v\n",
				s.Name, s.VCPUs, s.MemoryGiB, s.GPUs, od, sp, sav,
				s.CPUArch, s.Family,
				s.AcceleratedNetworking, s.PremiumIO,
				s.LocalTempDiskGiB, s.SpotCapable,
			)
		} else {
			fmt.Fprintf(tw, "%s\t%d\t%.1f\t%d\t%s\t%s\t%s\n",
				s.Name, s.VCPUs, s.MemoryGiB, s.GPUs, od, sp, sav,
			)
		}
	}
	return tw.Flush()
}

func fmtPrice(p float64) string {
	if p == 0 {
		return "-"
	}
	return fmt.Sprintf("%.4f", p)
}

func fmtSaving(pct float64) string {
	if pct == 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", pct)
}

func spotSaving(onDemand, spot float64) float64 {
	if onDemand == 0 || spot == 0 {
		return 0
	}
	return (1 - spot/onDemand) * 100
}
