package main

import (
	"encoding/json"
	"testing"
)

func TestWireContract(t *testing.T) {
	for _, tc := range []struct {
		method, request string
		ok              bool
	}{
		{"plugin.register", `{"schema_version":10}`, true},
		{"plugin.reconfigure", `{"schema_version":10}`, true},
		{"plugin.register", `{"schema_version":9}`, false},
		{"plugin.register", `{`, false},
		{"management.register", `{}`, true},
		{"management.handle", `{"Method":"GET","Path":"/open"}`, true},
		{"unknown", `{}`, false},
	} {
		raw, err := handleMethod(tc.method, []byte(tc.request))
		if err != nil {
			t.Fatal(err)
		}
		var envelope struct {
			OK     bool
			Result json.RawMessage
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.OK != tc.ok {
			t.Fatalf("%s: %s", tc.method, raw)
		}
	}
}

func TestResourceHasNoMutation(t *testing.T) {
	raw, err := handleMethod("management.handle", []byte(`{"Method":"POST"}`))
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct{ Result struct{ StatusCode int } }
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Result.StatusCode != 405 {
		t.Fatalf("unexpected response: %s", raw)
	}
}
