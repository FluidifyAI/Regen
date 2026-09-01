package llm

import (
	"context"
	"testing"
)

func TestCallMetaFromContext_ReturnsZeroValue_WhenNotSet(t *testing.T) {
	got := CallMetaFromContext(context.Background())
	if got != (CallMeta{}) {
		t.Errorf("CallMetaFromContext(bare ctx) = %+v, want zero value", got)
	}
}

func TestCallMetaFromContext_ReturnsWhatWasSet(t *testing.T) {
	meta := CallMeta{AgentName: "Incident Summarizer", IncidentID: "inc-123"}
	ctx := WithCallMeta(context.Background(), meta)

	got := CallMetaFromContext(ctx)
	if got != meta {
		t.Errorf("CallMetaFromContext(ctx) = %+v, want %+v", got, meta)
	}
}
