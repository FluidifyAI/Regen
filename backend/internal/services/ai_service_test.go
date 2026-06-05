package services_test

import (
	"testing"

	"github.com/FluidifyAI/Regen/backend/internal/services"
)

func TestAIService_Model_returnsConfiguredModel(t *testing.T) {
	svc := services.NewAIService("openai", "sk-test", "gpt-4o", 1000, 2000, "")
	if svc.Model() != "gpt-4o" {
		t.Errorf("Model() = %q, want %q", svc.Model(), "gpt-4o")
	}
}

func TestAIService_Model_updatesAfterReload(t *testing.T) {
	svc := services.NewAIService("openai", "sk-test", "gpt-4o", 1000, 2000, "")
	svc.Reload("openai", "sk-test", "gpt-4o-mini", "")
	if svc.Model() != "gpt-4o-mini" {
		t.Errorf("Model() after Reload = %q, want %q", svc.Model(), "gpt-4o-mini")
	}
}
