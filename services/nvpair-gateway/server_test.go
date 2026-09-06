// SPDX-FileCopyrightText: Copyright (c) 2026 AI Unlock Innovations Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newGatewayRequest(method, target string, body io.Reader) *http.Request {
	request := httptest.NewRequest(method, target, body)
	request.Host = "127.0.0.1:14322"
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}

func TestSpeechEndpointRequiresJSONContentType(t *testing.T) {
	hits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		hits++
		response.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	gateway := NewGateway(Config{Providers: []Provider{{
		ID: "tts", Name: "TTS", ProviderType: "custom-http", BaseURL: upstream.URL,
		Capabilities: []string{"tts"}, Routes: map[string]Route{"tts": {Method: "POST", Path: "/tts"}},
	}}}, upstream.Client())

	request := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", bytes.NewBufferString(`{"input":"hello"}`))
	request.Host = "192.168.1.10:14322"
	request.Header.Set("Content-Type", "text/plain")
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)
	if response.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", response.Code)
	}
	if hits != 0 {
		t.Fatalf("upstream received %d rejected requests, want 0", hits)
	}
}

func TestSpeechEndpointRejectsTrailingJSON(t *testing.T) {
	gateway := NewGateway(Config{Providers: []Provider{{
		ID: "tts", Name: "TTS", ProviderType: "custom-http", BaseURL: "http://127.0.0.1:1",
		Capabilities: []string{"tts"}, Routes: map[string]Route{"tts": {Method: "POST", Path: "/tts"}},
	}}}, http.DefaultClient)
	request := newGatewayRequest(http.MethodPost, "/v1/audio/speech", bytes.NewBufferString(`{"input":"hello"} {"extra":true}`))
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want trailing JSON rejected", response.Code)
	}
}

func TestSpeechEndpointRejectsMissingInputAndUnresolvedMapping(t *testing.T) {
	gateway := NewGateway(Config{Providers: []Provider{{
		ID: "tts", Name: "TTS", ProviderType: "custom-http", BaseURL: "http://127.0.0.1:1",
		Capabilities: []string{"tts"}, Routes: map[string]Route{"tts": {
			Method: "POST", Path: "/tts", RequestMapping: map[string]any{"text": "{{input}}", "speaker": "{{missing}}"},
		}},
	}}}, http.DefaultClient)
	for _, body := range []string{`{}`, `{"input":"hello"}`} {
		request := newGatewayRequest(http.MethodPost, "/v1/audio/speech", bytes.NewBufferString(body))
		response := httptest.NewRecorder()
		gateway.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d, want 400", body, response.Code)
		}
	}
}

func TestProviderRedirectDoesNotForwardConfiguredHeaders(t *testing.T) {
	leaked := ""
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		leaked = request.Header.Get("X-API-Key")
		response.WriteHeader(http.StatusOK)
	}))
	defer redirectTarget.Close()
	provider := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, redirectTarget.URL, http.StatusTemporaryRedirect)
	}))
	defer provider.Close()

	gateway := NewGateway(Config{Providers: []Provider{{
		ID: "redirecting", Name: "Redirecting", ProviderType: "custom-http", BaseURL: provider.URL,
		Capabilities: []string{"tts"}, Headers: map[string]string{"X-API-Key": "local-secret"},
		Routes: map[string]Route{"tts": {Method: "POST", Path: "/tts"}},
	}}}, http.DefaultClient)
	request := newGatewayRequest(http.MethodPost, "/v1/audio/speech", bytes.NewBufferString(`{"input":"hello"}`))
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want provider redirect rejected as bad gateway", response.Code)
	}
	if leaked != "" {
		t.Fatalf("configured provider header leaked to redirect target: %q", leaked)
	}
}

func TestGatewayHealthEndpoint(t *testing.T) {
	gateway := NewGateway(Config{}, http.DefaultClient)
	request := newGatewayRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("health response = (%d, %q), want status ok", response.Code, response.Body.String())
	}
}

func TestHealthChecksKeepUnhealthyProviderOutOfRouting(t *testing.T) {
	failedHits := 0
	unhealthy := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/health" {
			response.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
		failedHits++
		response.WriteHeader(http.StatusOK)
	}))
	defer unhealthy.Close()

	healthy := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/health" {
			response.WriteHeader(http.StatusOK)
			return
		}
		response.Header().Set("Content-Type", "audio/wav")
		_, _ = response.Write([]byte("healthy-audio"))
	}))
	defer healthy.Close()

	providers := []Provider{
		{ID: "bad", Name: "Bad", ProviderType: "custom-http", BaseURL: unhealthy.URL, HealthCheck: "/health", Capabilities: []string{"tts"}, Routes: map[string]Route{"tts": {Method: "POST", Path: "/tts"}}},
		{ID: "good", Name: "Good", ProviderType: "custom-http", BaseURL: healthy.URL, HealthCheck: "/health", Capabilities: []string{"tts"}, Routes: map[string]Route{"tts": {Method: "POST", Path: "/tts"}}},
	}
	gateway := NewGateway(Config{Providers: providers}, healthy.Client())
	gateway.RefreshHealth(context.Background())

	request := newGatewayRequest(http.MethodPost, "/v1/audio/speech", bytes.NewBufferString(`{"input":"hello"}`))
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "healthy-audio" {
		t.Fatalf("gateway response = (%d, %q), want healthy provider response", response.Code, response.Body.String())
	}
	if failedHits != 0 {
		t.Fatalf("unhealthy provider received %d inference requests, want 0", failedHits)
	}
}

func TestSpeechEndpointMapsAndForwardsToCustomHTTPProvider(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/tts" {
			t.Fatalf("upstream path = %q, want /tts", request.URL.Path)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer local-secret" {
			t.Fatalf("authorization = %q, want configured credential", got)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if body["text"] != "สวัสดีครับ" || body["speaker"] != "mor1" || body["speed"] != 1.25 {
			t.Fatalf("mapped body = %#v", body)
		}
		response.Header().Set("Content-Type", "audio/mpeg")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("fake-mp3"))
	}))
	defer upstream.Close()

	gateway := NewGateway(Config{Providers: []Provider{{
		ID: "tts-main", Name: "TTS", ProviderType: "custom-http", BaseURL: upstream.URL,
		Capabilities: []string{"tts"}, Headers: map[string]string{"Authorization": "Bearer local-secret"},
		Routes: map[string]Route{"tts": {
			Method: "POST", Path: "/tts", ResponseType: "audio",
			RequestMapping: map[string]any{"text": "{{input}}", "speaker": "{{voice}}", "speed": "{{speed}}"},
		}},
	}}}, upstream.Client())

	request := newGatewayRequest(http.MethodPost, "/v1/audio/speech", bytes.NewBufferString(`{"model":"qwen3-tts","input":"สวัสดีครับ","voice":"mor1","speed":1.25}`))
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "fake-mp3" {
		t.Fatalf("gateway response = (%d, %q), want (200, fake-mp3)", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "audio/mpeg" {
		t.Fatalf("content type = %q, want audio/mpeg", got)
	}
}

func TestCapabilitiesEndpointReturnsSortedDynamicCapabilities(t *testing.T) {
	gateway := NewGateway(Config{Providers: []Provider{
		{ID: "one", Capabilities: []string{"tts", "music", "voice-clone"}, Routes: map[string]Route{"tts": {}, "music": {}}},
		{ID: "two", Capabilities: []string{"llm", "tts"}, Routes: map[string]Route{"llm": {}}, Disabled: true},
	}}, http.DefaultClient)

	request := newGatewayRequest(http.MethodGet, "/v1/capabilities", nil)
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	var body struct {
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := []string{"music", "tts"}
	if len(body.Data) != len(want) || body.Data[0] != want[0] || body.Data[1] != want[1] {
		t.Fatalf("capabilities = %#v, want %#v", body.Data, want)
	}
}

func TestProvidersEndpointRedactsCredentialHeaders(t *testing.T) {
	gateway := NewGateway(Config{Providers: []Provider{{
		ID:           "tts-main",
		Name:         "AI UNLOCK TTS",
		ProviderType: "custom-http",
		BaseURL:      "http://127.0.0.1:9000",
		Capabilities: []string{"tts", "voice-clone"},
		Headers:      map[string]string{"Authorization": "Bearer secret-value"},
	}}}, http.DefaultClient)

	request := newGatewayRequest(http.MethodGet, "/v1/providers", nil)
	response := httptest.NewRecorder()
	gateway.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if strings.Contains(response.Body.String(), "secret-value") {
		t.Fatal("providers response leaked a credential header")
	}
	if strings.Contains(response.Body.String(), "baseUrl") || strings.Contains(response.Body.String(), "healthCheck") {
		t.Fatal("providers response leaked provider endpoint details")
	}
	var body struct {
		Data []struct {
			ID           string   `json:"id"`
			Capabilities []string `json:"capabilities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].ID != "tts-main" {
		t.Fatalf("providers = %#v, want tts-main", body.Data)
	}
}
