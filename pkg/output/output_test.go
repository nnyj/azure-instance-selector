package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nnyj/azure-instance-selector/pkg/selector"
)

func TestSpotSaving(t *testing.T) {
	cases := []struct {
		onDemand, spot, want float64
	}{
		{0.20, 0.05, 75},
		{0.20, 0.20, 0},
		{0, 0.05, 0},
		{0.20, 0, 0},
	}
	for _, c := range cases {
		if got := spotSaving(c.onDemand, c.spot); got != c.want {
			t.Errorf("spotSaving(%v, %v) = %v, want %v", c.onDemand, c.spot, got, c.want)
		}
	}
}

func TestFmtPrice_ZeroReturnsDash(t *testing.T) {
	if got := fmtPrice(0); got != "-" {
		t.Errorf("fmtPrice(0) = %q, want \"-\"", got)
	}
	if got := fmtPrice(0.1234); got != "0.1234" {
		t.Errorf("fmtPrice(0.1234) = %q, want \"0.1234\"", got)
	}
}

func TestRenderOneLine(t *testing.T) {
	skus := []selector.VmSku{
		{Name: "Standard_D4s_v5"},
		{Name: "Standard_A0"},
	}
	var buf bytes.Buffer
	if err := renderOneLine(&buf, skus); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "Standard_D4s_v5 Standard_A0" {
		t.Errorf("renderOneLine = %q, want \"Standard_D4s_v5 Standard_A0\"", got)
	}
}

func TestRenderTable_HeaderAndRow(t *testing.T) {
	skus := []selector.VmSku{
		{Name: "Standard_D4s_v5", VCPUs: 4, MemoryGiB: 16},
	}
	prices := map[string]selector.Prices{
		"Standard_D4s_v5": {OnDemandHr: 0.192, SpotHr: 0.048},
	}
	var buf bytes.Buffer
	if err := renderTable(&buf, skus, prices, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"NAME", "VCPUs", "MEM GiB", "ON-DEMAND $/hr", "SPOT $/hr", "SPOT SAVE%",
		"Standard_D4s_v5", "0.1920", "0.0480", "75%",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("renderTable output missing %q\n%s", want, out)
		}
	}
}
