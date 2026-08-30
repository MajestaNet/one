package inference

import (
	"strings"
	"testing"
)

func TestModelForMode(t *testing.T) {
	id, err := ModelForMode(ModeDev)
	if err != nil || id != "openai-gpt-oss-20b" {
		t.Fatalf("dev: %q %v", id, err)
	}
	id, err = ModelForMode(ModeStandard)
	if err != nil || id != "llama-4-maverick" {
		t.Fatalf("standard: %q %v", id, err)
	}
	id, err = ModelForMode(ModePro)
	if err != nil || id != "openai-gpt-oss-120b" {
		t.Fatalf("pro: %q %v", id, err)
	}
	if _, err := ModelForMode(Mode("nope")); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateMode(ModeStandard); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMode(Mode("nope")); err == nil {
		t.Fatal("expected invalid mode")
	}
}

func TestStatusJSONModelIDAlias(t *testing.T) {
	mode := ModeStandard
	out := StatusJSON(&InstallConfig{
		ActiveSource: SourceDigitalOcean,
		DOEnabled:    true,
		DOMode:       &mode,
	}, nil, true)
	if out["modelId"] != out["doModelId"] {
		t.Fatalf("modelId=%v doModelId=%v", out["modelId"], out["doModelId"])
	}
	if out["modelId"] != "llama-4-maverick" {
		t.Fatalf("modelId=%v", out["modelId"])
	}
	modes, _ := out["doModeModels"].(map[string]string)
	if modes["standard"] != "llama-4-maverick" || modes["dev"] != "openai-gpt-oss-20b" || modes["pro"] != "openai-gpt-oss-120b" {
		t.Fatalf("doModeModels=%v", modes)
	}
	if out["prepaid"] != true || out["billingNotice"] == "" {
		t.Fatalf("billing fields: prepaid=%v notice=%v", out["prepaid"], out["billingNotice"])
	}
}

func TestFormatRouteErrorNotConfigured(t *testing.T) {
	code, msg := FormatRouteError(ErrNotConfigured)
	if code != "INFERENCE_NOT_CONFIGURED" {
		t.Fatalf("code=%s", code)
	}
	if strings.Contains(strings.ToLower(msg), "settings") {
		t.Fatalf("must not mention Settings: %s", msg)
	}
	if !strings.Contains(msg, "/metadata/v1/inference/providers") || !strings.Contains(msg, "/deploy/v1/cloud/inference") {
		t.Fatalf("expected Metadata/Deploy paths: %s", msg)
	}
}

func TestBuildAgentMessages(t *testing.T) {
	msgs := BuildAgentMessages("Be brief.", "List accounts", map[string]any{"userMessage": "Hi"})
	if len(msgs) != 2 || msgs[0].Role != "system" || msgs[1].Content != "Hi" {
		t.Fatalf("%+v", msgs)
	}
	if !contains(msgs[0].Content, "oneEffects") {
		t.Fatalf("expected oneEffects coaching in system message")
	}
}

func TestChatURL(t *testing.T) {
	if got := chatURL("https://api.openai.com/v1"); got != "https://api.openai.com/v1/chat/completions" {
		t.Fatalf("%s", got)
	}
	if got := chatURL("https://x/v1/chat/completions"); got != "https://x/v1/chat/completions" {
		t.Fatalf("%s", got)
	}
}

func TestValidateProviderBaseURLRejectsHTTP(t *testing.T) {
	if err := ValidateProviderBaseURL("http://api.openai.com/v1", false); err == nil {
		t.Fatal("expected http reject")
	}
}

func TestValidateProviderBaseURLDevAllowsOllama(t *testing.T) {
	if err := ValidateProviderBaseURL("http://127.0.0.1:11434/v1", true); err != nil {
		t.Fatalf("dev local ollama: %v", err)
	}
	if err := ValidateProviderBaseURL("http://localhost:11434/v1", true); err != nil {
		t.Fatalf("localhost: %v", err)
	}
	if err := ValidateProviderBaseURL("http://host.docker.internal:11434/v1", true); err != nil {
		t.Fatalf("docker host: %v", err)
	}
	if err := ValidateProviderBaseURL("http://127.0.0.1:11434/v1", false); err == nil {
		t.Fatal("production must still reject loopback http")
	}
	if err := ValidateProviderBaseURL("http://10.1.2.3:11434/v1", true); err == nil {
		t.Fatal("dev mode must not open arbitrary private HTTP")
	}
}
