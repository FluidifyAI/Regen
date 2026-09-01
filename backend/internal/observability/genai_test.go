package observability

import (
	"os"
	"testing"
)

func clearGenAIContentEnv(t *testing.T) {
	t.Helper()
	old, had := os.LookupEnv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT")
	os.Unsetenv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT")
	t.Cleanup(func() {
		if had {
			os.Setenv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT", old)
		} else {
			os.Unsetenv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT")
		}
	})
}

func TestGenAIPromptRecordingEnabled_DefaultsToFalse(t *testing.T) {
	clearGenAIContentEnv(t)
	if GenAIPromptRecordingEnabled() {
		t.Error("expected false with OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT unset")
	}
}

func TestGenAIPromptRecordingEnabled_TrueWhenExplicitlySet(t *testing.T) {
	clearGenAIContentEnv(t)
	os.Setenv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT", "true")
	if !GenAIPromptRecordingEnabled() {
		t.Error("expected true with OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=true")
	}
}
