package agentloop

import (
	"encoding/json"
	"strings"
)

var secretArgKeys = map[string]struct{}{
	"password": {}, "secret": {}, "apikey": {}, "api_key": {}, "token": {},
	"authorization": {}, "clientsecret": {}, "client_secret": {}, "access_token": {},
}

func redactArgs(args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(args))
	for k, v := range args {
		if isSecretKey(k) {
			out[k] = "[redacted]"
			continue
		}
		if nested, ok := v.(map[string]any); ok {
			out[k] = redactArgs(nested)
			continue
		}
		out[k] = v
	}
	return out
}

func isSecretKey(k string) bool {
	n := strings.ToLower(strings.TrimSpace(k))
	n = strings.ReplaceAll(n, "-", "")
	n = strings.ReplaceAll(n, "_", "")
	if _, ok := secretArgKeys[n]; ok {
		return true
	}
	if _, ok := secretArgKeys[strings.ToLower(strings.TrimSpace(k))]; ok {
		return true
	}
	return false
}

func redactToolCallPayload(name string, args map[string]any) map[string]any {
	return map[string]any{"name": name, "arguments": redactArgs(args)}
}

func truncateForModel(s string) string {
	if len(s) <= MaxToolResultBytes {
		return s
	}
	return s[:MaxToolResultBytes] + "\n…[truncated]"
}

func jsonLimit(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return map[string]any{"error": "unserializable result"}
	}
	if len(b) <= MaxToolResultBytes {
		return v
	}
	var trimmed any
	s := truncateForModel(string(b))
	if json.Unmarshal([]byte(s), &trimmed) == nil {
		return trimmed
	}
	return s
}
