package telegram

import "testing"

func TestExtractTotalPrice_OverallPriceLine(t *testing.T) {
	text := "CPU - 170$\nGPU - 300$\nOverall price: 470$"
	if got := extractTotalPrice(text); got != "470$" {
		t.Fatalf("extractTotalPrice() = %q, want %q", got, "470$")
	}
}

func TestExtractTotalPrice_BulletPrefixTotal(t *testing.T) {
	text := "* Jami: 470$\nCPU - 170$\nGPU - 300$"
	if got := extractTotalPrice(text); got != "470$" {
		t.Fatalf("extractTotalPrice() = %q, want %q", got, "470$")
	}
}

func TestSumPriceLines_IgnoresConvertedSoM_WhenUSDPresent(t *testing.T) {
	text := "CPU - 170$ (~2 125 000 so'm)\nGPU - 300$ (~3 750 000 so'm)\nRAM - 50$ (~625 000 so'm)"
	if got := sumPriceLines(text); got != "520$" {
		t.Fatalf("sumPriceLines() = %q, want %q", got, "520$")
	}
}

func TestSumPriceLines_ParsesThousandSeparators(t *testing.T) {
	text := "GPU - 1,400.00$\nCPU - 170$"
	if got := sumPriceLines(text); got != "1570$" {
		t.Fatalf("sumPriceLines() = %q, want %q", got, "1570$")
	}
}

func TestSumPriceLines_SkipsTotalLines(t *testing.T) {
	text := "CPU - 170$\nGPU - 300$\nJami: 470$"
	if got := sumPriceLines(text); got != "470$" {
		t.Fatalf("sumPriceLines() = %q, want %q", got, "470$")
	}
}
