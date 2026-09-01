package observability

import "os"

// GenAIPromptRecordingEnabled reports whether LLM instrumentation should
// record prompt and completion text on spans, gated by the env var the
// upstream OTel GenAI instrumentation convention defines for this exact
// purpose (OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT). The full
// upstream spec also accepts NO_CONTENT/SPAN_ONLY/EVENT_ONLY/SPAN_AND_EVENT;
// Regen only needs the boolean case ("local debugging only" per REG-12) so
// this implements just the true/false gate, matching SQLStatementRecordingEnabled's
// shape for the same reason (query text and prompt text are both a data-egress
// path with the same exposure as an OTLP export).
//
// Off by default: incident data — including whatever an on-call engineer
// pasted into a summarize/postmortem/@mention prompt — is customer data.
func GenAIPromptRecordingEnabled() bool {
	return os.Getenv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT") == "true"
}
