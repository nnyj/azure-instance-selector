package pricing

import "testing"

func TestClassifyPrices_SpotVsOnDemand(t *testing.T) {
	items := []retailItem{
		{ArmSkuName: "Standard_D4s_v5", SkuName: "D4s v5 Spot", ProductName: "Virtual Machines D Series", UnitPrice: 0.05},
		{ArmSkuName: "Standard_D4s_v5", SkuName: "D4s v5", MeterName: "D4s v5", ProductName: "Virtual Machines D Series", UnitPrice: 0.20},
	}
	got := classifyPrices(items, "linux")
	p := got["Standard_D4s_v5"]
	if p.SpotHr != 0.05 {
		t.Errorf("SpotHr = %v, want 0.05", p.SpotHr)
	}
	if p.OnDemandHr != 0.20 {
		t.Errorf("OnDemandHr = %v, want 0.20", p.OnDemandHr)
	}
}

func TestClassifyPrices_SpotDetectedViaMeterName(t *testing.T) {
	items := []retailItem{
		{ArmSkuName: "Standard_D4s_v5", SkuName: "D4s v5", MeterName: "D4s v5 Spot", ProductName: "Virtual Machines D Series", UnitPrice: 0.03},
	}
	got := classifyPrices(items, "linux")
	if got["Standard_D4s_v5"].SpotHr != 0.03 {
		t.Errorf("meterName spot detection: SpotHr = %v, want 0.03", got["Standard_D4s_v5"].SpotHr)
	}
}

func TestClassifyPrices_LowPriorityRowsSkipped(t *testing.T) {
	items := []retailItem{
		{ArmSkuName: "Standard_D4s_v5", SkuName: "D4s v5 Low Priority", ProductName: "Virtual Machines D Series", UnitPrice: 0.01},
		{ArmSkuName: "Standard_D4s_v5", MeterName: "D4s v5 Low Priority", ProductName: "Virtual Machines D Series", UnitPrice: 0.01},
	}
	got := classifyPrices(items, "linux")
	if _, ok := got["Standard_D4s_v5"]; ok {
		t.Error("low priority rows should be skipped")
	}
}

func TestClassifyPrices_WindowsVsLinuxFiltering(t *testing.T) {
	items := []retailItem{
		{ArmSkuName: "Standard_D4s_v5", SkuName: "D4s v5", ProductName: "Virtual Machines D Series Windows", UnitPrice: 0.30},
		{ArmSkuName: "Standard_D4s_v5", SkuName: "D4s v5", ProductName: "Virtual Machines D Series", UnitPrice: 0.20},
	}
	if p := classifyPrices(items, "linux")["Standard_D4s_v5"]; p.OnDemandHr != 0.20 {
		t.Errorf("linux OnDemandHr = %v, want 0.20", p.OnDemandHr)
	}
	if p := classifyPrices(items, "windows")["Standard_D4s_v5"]; p.OnDemandHr != 0.30 {
		t.Errorf("windows OnDemandHr = %v, want 0.30", p.OnDemandHr)
	}
}

func TestClassifyPrices_MinPriceWinsOnDuplicate(t *testing.T) {
	items := []retailItem{
		{ArmSkuName: "Standard_D4s_v5", SkuName: "D4s v5", ProductName: "Virtual Machines D Series", UnitPrice: 0.30},
		{ArmSkuName: "Standard_D4s_v5", SkuName: "D4s v5", ProductName: "Virtual Machines D Series", UnitPrice: 0.20},
	}
	got := classifyPrices(items, "linux")
	if got["Standard_D4s_v5"].OnDemandHr != 0.20 {
		t.Errorf("min-price-wins: got %v, want 0.20", got["Standard_D4s_v5"].OnDemandHr)
	}
}

func TestClassifyPrices_UnitPriceZeroFallsBackToRetailPrice(t *testing.T) {
	items := []retailItem{
		{ArmSkuName: "Standard_D4s_v5", SkuName: "D4s v5", ProductName: "Virtual Machines D Series", UnitPrice: 0, RetailPrice: 0.25},
	}
	got := classifyPrices(items, "linux")
	if got["Standard_D4s_v5"].OnDemandHr != 0.25 {
		t.Errorf("retail fallback: got %v, want 0.25", got["Standard_D4s_v5"].OnDemandHr)
	}
}

func TestClassifyPrices_EmptyArmSkuNameSkipped(t *testing.T) {
	items := []retailItem{
		{ArmSkuName: "", SkuName: "D4s v5", ProductName: "Virtual Machines D Series", UnitPrice: 0.20},
	}
	got := classifyPrices(items, "linux")
	if len(got) != 0 {
		t.Errorf("empty armSkuName should be skipped; got %d entries", len(got))
	}
}
