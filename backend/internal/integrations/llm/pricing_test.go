package llm

import "testing"

func TestEstimateCostUSD_KnownModel_ComputesFromTokenCounts(t *testing.T) {
	// gpt-4o: $2.50 / 1M input, $10.00 / 1M output (per pricingTable).
	got := estimateCostUSD("gpt-4o", 1_000_000, 1_000_000)
	want := 2.50 + 10.00
	if got != want {
		t.Errorf("estimateCostUSD(gpt-4o, 1M, 1M) = %v, want %v", got, want)
	}
}

func TestEstimateCostUSD_UnknownModel_ReturnsZero(t *testing.T) {
	got := estimateCostUSD("some-model-not-in-the-table", 1000, 1000)
	if got != 0 {
		t.Errorf("estimateCostUSD(unknown model) = %v, want 0", got)
	}
}

func TestEstimateCostUSD_ZeroTokens_ReturnsZero(t *testing.T) {
	got := estimateCostUSD("gpt-4o", 0, 0)
	if got != 0 {
		t.Errorf("estimateCostUSD(gpt-4o, 0, 0) = %v, want 0", got)
	}
}
