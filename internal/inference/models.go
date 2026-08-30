package inference

import "fmt"

// Mode is the Native DigitalOcean Inference tier.
type Mode string

const (
	ModeDev      Mode = "dev"
	ModeStandard Mode = "standard"
	ModePro      Mode = "pro"
)

// DOModeModels is the single retune point for Native DigitalOcean Serverless
// Inference model IDs (inference-build-plan choice 2C). Swap IDs here only;
// they are resolved at Resolve time and are not stored. No migration.
//
// Catalog (docs.digitalocean.com/products/inference/details/models, 2026-08):
// Dev and Pro remain listed serverless OSS models; Standard is llama-4-maverick
// (llama3.3-70b-instruct is no longer on the serverless catalog).
var DOModeModels = map[Mode]string{
	ModeDev:      "openai-gpt-oss-20b",
	ModeStandard: "llama-4-maverick",
	ModePro:      "openai-gpt-oss-120b",
}

const (
	DOInferenceBaseURL = "https://inference.do-ai.run/v1"
	// BillingNotice is returned on Deploy GET|PUT /cloud/inference (prepaid, customer DO bill).
	BillingNotice = "Inference and hosting are billed by DigitalOcean on your account. DigitalOcean Serverless Inference is prepaid: keep a positive prepaid balance or requests are suspended."
)

// ValidateMode returns an error if mode is unknown.
func ValidateMode(m Mode) error {
	if _, ok := DOModeModels[m]; !ok {
		return fmt.Errorf("inference: invalid mode %q", m)
	}
	return nil
}

// ModelForMode returns the provisional model id for a DO tier.
func ModelForMode(m Mode) (string, error) {
	id, ok := DOModeModels[m]
	if !ok {
		return "", fmt.Errorf("inference: invalid mode %q", m)
	}
	return id, nil
}
