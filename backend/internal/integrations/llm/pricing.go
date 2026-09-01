package llm

// modelPricing holds USD list price per 1M tokens, priced separately for
// input and output since providers charge different rates for each.
//
// This is Regen's own OSS estimate table, used to tag the estimated cost of
// a completion directly on its trace span. It is a separate computation from
// whatever a Pro CostTracker.RecordUsage implementation charges internally —
// there is no shared code path between the two (Pro is a private, frozen
// repo; see CLAUDE.md's two-repo model) — but both are fed the same Model,
// PromptTokens, and CompletionTokens for a given call, so as long as this
// table's prices match the provider's real list price, the span's estimate
// and the Pro cost endpoint's total agree numerically.
//
// Prices are current as of the provider's published rates at the time this
// table was written and will drift as providers change pricing; update here
// when they do. An unlisted model returns 0 rather than guessing.
type modelPricing struct {
	InputPer1M  float64
	OutputPer1M float64
}

var pricingTable = map[string]modelPricing{
	// OpenAI
	"gpt-4o":      {InputPer1M: 2.50, OutputPer1M: 10.00},
	"gpt-4o-mini": {InputPer1M: 0.15, OutputPer1M: 0.60},
	"gpt-4-turbo": {InputPer1M: 10.00, OutputPer1M: 30.00},
	"o1":          {InputPer1M: 15.00, OutputPer1M: 60.00},
	"o1-mini":     {InputPer1M: 1.10, OutputPer1M: 4.40},

	// Anthropic
	"claude-3-5-sonnet-20241022": {InputPer1M: 3.00, OutputPer1M: 15.00},
	"claude-3-5-haiku-20241022":  {InputPer1M: 0.80, OutputPer1M: 4.00},
	"claude-3-opus-20240229":     {InputPer1M: 15.00, OutputPer1M: 75.00},

	// Ollama models are self-hosted and intentionally absent from this
	// table — they have no per-token USD cost, and always estimate 0.
}

// estimateCostUSD returns the estimated USD cost of one completion given its
// token counts, or 0 if model has no configured pricing — the same "unknown
// model costs nothing" fallback enterprise.CostTracker documents for
// RecordUsage, so an unpriced model reads the same way in both places.
func estimateCostUSD(model string, promptTokens, completionTokens int) float64 {
	p, ok := pricingTable[model]
	if !ok {
		return 0
	}
	return (float64(promptTokens)/1_000_000)*p.InputPer1M + (float64(completionTokens)/1_000_000)*p.OutputPer1M
}
