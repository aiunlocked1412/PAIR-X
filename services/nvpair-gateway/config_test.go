// SPDX-FileCopyrightText: Copyright (c) 2026 AI Unlock Innovations Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

func TestLoadConfigRejectsCredentialBearingBaseURL(t *testing.T) {
	for _, baseURL := range []string{
		"http://user:password@127.0.0.1:9000",
		"http://127.0.0.1:9000?api_key=secret",
	} {
		configJSON := `{"providers":[{"id":"unsafe","name":"Unsafe","providerType":"custom-http","baseUrl":"` + baseURL + `","capabilities":["tts"]}]}`
		_, err := LoadConfig(strings.NewReader(configJSON))
		if err == nil || !strings.Contains(err.Error(), "must not contain credentials or a query") {
			t.Fatalf("baseUrl %q error = %v, want credential/query rejection", baseURL, err)
		}
	}
}

func TestLoadConfigAcceptsLANListener(t *testing.T) {
	configJSON := `{"listen":"0.0.0.0:14322","providers":[]}`
	config, err := LoadConfig(strings.NewReader(configJSON))
	if err != nil || config.Listen != "0.0.0.0:14322" {
		t.Fatalf("config = %#v, error = %v, want LAN listener accepted", config, err)
	}
}

func TestLoadConfigAcceptsDynamicProviderTypesAndCapabilities(t *testing.T) {
	configJSON := `{
		"providers": [{
			"id": "music-main",
			"name": "Local Music",
			"providerType": "future-provider-type",
			"baseUrl": "http://127.0.0.1:9000",
			"capabilities": ["music", "tts"],
			"routes": {
				"tts": {"method": "POST", "path": "/speak"}
			}
		}]
	}`

	config, err := LoadConfig(strings.NewReader(configJSON))
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if got := config.Providers[0].ProviderType; got != "future-provider-type" {
		t.Fatalf("provider type = %q, want future-provider-type", got)
	}
	if got := config.Providers[0].Capabilities[0]; got != "music" {
		t.Fatalf("first capability = %q, want music", got)
	}
}

func TestLoadConfigRejectsDuplicateProviderIDs(t *testing.T) {
	configJSON := `{
		"providers": [
			{"id":"same","name":"One","providerType":"custom-http","baseUrl":"http://127.0.0.1:9001","capabilities":["tts"]},
			{"id":"same","name":"Two","providerType":"custom-http","baseUrl":"http://127.0.0.1:9002","capabilities":["tts"]}
		]
	}`

	_, err := LoadConfig(strings.NewReader(configJSON))
	if err == nil || !strings.Contains(err.Error(), "duplicate provider id") {
		t.Fatalf("error = %v, want duplicate provider id error", err)
	}
}

func TestLoadConfigRejectsUnsupportedRetryConfiguration(t *testing.T) {
	configJSON := `{"providers":[{"id":"tts","name":"TTS","providerType":"custom-http","baseUrl":"http://127.0.0.1:9000","capabilities":["tts"],"maxRetries":2}]}`
	_, err := LoadConfig(strings.NewReader(configJSON))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %v, want unsupported field rejection", err)
	}
}
