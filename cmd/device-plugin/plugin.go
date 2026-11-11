package main

import (
	"encoding/json"
	"html"
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
			ConfigYAML    []byte `json:"config_yaml"`
		}
		if json.Unmarshal(request, &req) != nil || req.SchemaVersion != schemaVersion {
			return errorEnvelope("unsupported_schema", "requires CPA schema 10"), nil
		}
		if err := configure(req.ConfigYAML); err != nil {
			return errorEnvelope("configuration_error", err.Error()), nil
		}
		return okEnvelope(map[string]any{
			"schema_version": schemaVersion,
			"metadata":       map[string]any{"Name": "Device Usage Keeper", "Version": "0.1.0-dev", "Author": "NotRllyRn", "GitHubRepository": "https://github.com/NotRllyRn/cpa-device-usage-keeper", "ConfigFields": []map[string]string{{"Name": "listen", "Type": "string", "Description": "HTTP bind address, default 127.0.0.1:8788; put behind TLS reverse proxy"}, {"Name": "data_dir", "Type": "string", "Description": "Absolute private persistent directory (0700)"}, {"Name": "dashboard_url", "Type": "string", "Description": "Optional public HTTPS dashboard URL for CPA iframe"}}},
			"capabilities":   map[string]bool{"management_api": true},
		})
	case "management.register":
		return okEnvelope(map[string]any{"resources": []map[string]string{{"Path": "/open", "Menu": "Device Usage Keeper", "Description": "Device usage dashboard hosted by this plugin."}}})
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
			runtime.Lock()
			dashboardURL := runtime.dashboardURL
			runtime.Unlock()
			if dashboardURL != "" {
				body = `<!doctype html><html><meta charset="utf-8"><title>Device Usage Keeper</title><iframe title="Device Usage Keeper" referrerpolicy="no-referrer" style="position:fixed;inset:0;width:100%;height:100%;border:0" src="` + html.EscapeString(dashboardURL) + `"></iframe></html>`
			}
		}
		return okEnvelope(struct {
			StatusCode int
			Headers    http.Header
			Body       []byte
		}{
			status, http.Header{"Content-Type": {"text/html; charset=utf-8"}, "Cache-Control": {"no-store"}, "X-Content-Type-Options": {"nosniff"}, "Content-Security-Policy": {"default-src 'none'; frame-src https:; style-src 'unsafe-inline'; frame-ancestors 'self'"}}, []byte(body),
		})
	default:
		return errorEnvelope("unknown_method", "unsupported plugin method"), nil
	}
}

const page = `<!doctype html><html lang="en"><meta charset="utf-8"><title>Device Usage Keeper</title><h1>Device Usage Keeper</h1><p>Configure listen and data_dir to start the embedded device backend. Set dashboard_url to its reverse-proxied HTTPS address to embed the dashboard here.</p><p>Device uploads use separate upload-only tokens, not CPA credentials.</p><p>Independent fork of <a href="https://github.com/Willxup/cpa-usage-keeper">CPA Usage Keeper</a>.</p></html>`
