package main

import (
	"encoding/json"
	"net/http"
)

// This small wire adapter deliberately avoids importing CPA's full dependency
// graph. Contract pinned to CPA v8.0.63: ABI 1, JSON schema 10.
const schemaVersion = 10

func errorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(map[string]any{"ok": false, "error": map[string]string{"code": code, "message": message}})
	return raw
}

func okEnvelope(result any) ([]byte, error) {
	return json.Marshal(map[string]any{"ok": true, "result": result})
}

func handleMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case "plugin.register", "plugin.reconfigure":
		var req struct {
			SchemaVersion uint32 `json:"schema_version"`
		}
		if json.Unmarshal(request, &req) != nil || req.SchemaVersion != schemaVersion {
			return errorEnvelope("unsupported_schema", "requires CPA schema 10"), nil
		}
		return okEnvelope(map[string]any{
			"schema_version": schemaVersion,
			"metadata":       map[string]string{"Name": "Device Usage Keeper", "Version": "0.0.0", "Author": "NotRllyRn", "GitHubRepository": "https://github.com/NotRllyRn/cpa-device-usage-keeper"},
			"capabilities":   map[string]bool{"management_api": true},
		})
	case "management.register":
		return okEnvelope(map[string]any{"resources": []map[string]string{{"Path": "/open", "Menu": "Device Usage Keeper", "Description": "Native hosting proof; device ingestion is not available."}}})
	case "management.handle":
		var req struct {
			Method string
			Path   string
		}
		if json.Unmarshal(request, &req) != nil {
			return errorEnvelope("invalid_request", "invalid resource request"), nil
		}
		// The host owns path matching. Only a public, non-sensitive static page is
		// exposed in this proof. No state, credentials, usage, or mutations exist.
		status, body := http.StatusMethodNotAllowed, "method not allowed"
		if req.Method == http.MethodGet {
			status, body = http.StatusOK, page
		}
		return okEnvelope(struct {
			StatusCode int
			Headers    http.Header
			Body       []byte
		}{
			status, http.Header{"Content-Type": {"text/html; charset=utf-8"}, "Cache-Control": {"no-store"}, "X-Content-Type-Options": {"nosniff"}, "Content-Security-Policy": {"default-src 'none'; frame-ancestors 'self'"}}, []byte(body),
		})
	default:
		return errorEnvelope("unknown_method", "unsupported plugin method"), nil
	}
}

const page = `<!doctype html><html lang="en"><meta charset="utf-8"><title>Device Usage Keeper</title><h1>Device Usage Keeper</h1><p>Native plugin hosting proof.</p><p>Device ingestion and dashboard are not implemented. No usage data is exposed here.</p><p>Independent fork of <a href="https://github.com/Willxup/cpa-usage-keeper">CPA Usage Keeper</a>.</p></html>`
