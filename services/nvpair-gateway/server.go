// SPDX-FileCopyrightText: Copyright (c) 2026 AI Unlock Innovations Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Gateway struct {
	config   Config
	client   *http.Client
	statusMu sync.RWMutex
	status   map[string]string
}

type providerView struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	ProviderType string   `json:"providerType"`
	Capabilities []string `json:"capabilities"`
	Disabled     bool     `json:"disabled"`
	Status       string   `json:"status"`
}

func NewGateway(config Config, client *http.Client) *Gateway {
	if client == nil {
		client = http.DefaultClient
	}
	clientCopy := *client
	clientCopy.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	status := make(map[string]string, len(config.Providers))
	for _, provider := range config.Providers {
		if provider.Disabled {
			status[provider.ID] = "OFFLINE"
		} else if provider.HealthCheck == "" {
			status[provider.ID] = "ONLINE"
		} else {
			status[provider.ID] = "UNKNOWN"
		}
	}
	return &Gateway{config: config, client: &clientCopy, status: status}
}

func (gateway *Gateway) RefreshHealth(ctx context.Context) {
	for _, provider := range gateway.config.Providers {
		status := "OFFLINE"
		if !provider.Disabled {
			status = "ONLINE"
			if provider.HealthCheck != "" {
				target := strings.TrimRight(provider.BaseURL, "/") + "/" + strings.TrimLeft(provider.HealthCheck, "/")
				request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
				if err == nil {
					for name, value := range provider.Headers {
						request.Header.Set(name, value)
					}
					providerResponse, requestErr := gateway.client.Do(request)
					if requestErr != nil || providerResponse.StatusCode < 200 || providerResponse.StatusCode >= 300 {
						status = "OFFLINE"
					}
					if requestErr == nil {
						_, _ = io.Copy(io.Discard, providerResponse.Body)
						providerResponse.Body.Close()
					}
				} else {
					status = "OFFLINE"
				}
			}
		}
		gateway.statusMu.Lock()
		gateway.status[provider.ID] = status
		gateway.statusMu.Unlock()
	}
}

func (gateway *Gateway) providerStatus(id string) string {
	gateway.statusMu.RLock()
	defer gateway.statusMu.RUnlock()
	return gateway.status[id]
}

func (gateway *Gateway) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodPost {
		mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			writeJSON(response, http.StatusUnsupportedMediaType, map[string]any{"error": map[string]string{"message": "Content-Type must be application/json"}})
			return
		}
	}
	if request.Method == http.MethodGet && request.URL.Path == "/health" {
		writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/v1/providers" {
		gateway.listProviders(response)
		return
	}
	if request.Method == http.MethodGet && request.URL.Path == "/v1/capabilities" {
		gateway.listCapabilities(response)
		return
	}
	if request.Method == http.MethodPost && request.URL.Path == "/v1/audio/speech" {
		gateway.forwardCapability(response, request, "tts")
		return
	}
	http.NotFound(response, request)
}

func (gateway *Gateway) forwardCapability(response http.ResponseWriter, request *http.Request, capability string) {
	provider, route, ok := gateway.findProvider(capability)
	if !ok {
		writeJSON(response, http.StatusServiceUnavailable, map[string]any{"error": map[string]string{"message": "no provider available for " + capability}})
		return
	}
	var standardRequest map[string]any
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 16<<20))
	if err := decoder.Decode(&standardRequest); err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]any{"error": map[string]string{"message": "invalid JSON request"}})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSON(response, http.StatusBadRequest, map[string]any{"error": map[string]string{"message": "request must contain one JSON object"}})
		return
	}
	inputText, hasInput := standardRequest["input"].(string)
	if !hasInput || strings.TrimSpace(inputText) == "" {
		writeJSON(response, http.StatusBadRequest, map[string]any{"error": map[string]string{"message": "input is required"}})
		return
	}
	if err := validateMapping(route.RequestMapping, standardRequest); err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]any{"error": map[string]string{"message": err.Error()}})
		return
	}
	mapped := resolveMapping(route.RequestMapping, standardRequest)
	body, err := json.Marshal(mapped)
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]any{"error": map[string]string{"message": "request mapping failed"}})
		return
	}
	ctx := request.Context()
	if route.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(route.TimeoutSeconds)*time.Second)
		defer cancel()
	}
	method := route.Method
	if method == "" {
		method = http.MethodPost
	}
	target := strings.TrimRight(provider.BaseURL, "/") + "/" + strings.TrimLeft(route.Path, "/")
	upstreamRequest, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		writeJSON(response, http.StatusBadGateway, map[string]any{"error": map[string]string{"message": "provider request could not be created"}})
		return
	}
	upstreamRequest.Header.Set("Content-Type", "application/json")
	for name, value := range provider.Headers {
		upstreamRequest.Header.Set(name, value)
	}
	upstreamResponse, err := gateway.client.Do(upstreamRequest)
	if err != nil {
		writeJSON(response, http.StatusBadGateway, map[string]any{"error": map[string]string{"message": "provider request failed"}})
		return
	}
	defer upstreamResponse.Body.Close()
	if upstreamResponse.StatusCode >= 300 && upstreamResponse.StatusCode < 400 {
		_, _ = io.Copy(io.Discard, upstreamResponse.Body)
		writeJSON(response, http.StatusBadGateway, map[string]any{"error": map[string]string{"message": "provider redirect rejected"}})
		return
	}
	if contentType := upstreamResponse.Header.Get("Content-Type"); contentType != "" {
		response.Header().Set("Content-Type", contentType)
	}
	response.WriteHeader(upstreamResponse.StatusCode)
	_, _ = io.Copy(response, upstreamResponse.Body)
}

func (gateway *Gateway) findProvider(capability string) (Provider, Route, bool) {
	for _, provider := range gateway.config.Providers {
		if provider.Disabled {
			continue
		}
		if gateway.providerStatus(provider.ID) == "OFFLINE" {
			continue
		}
		route, hasRoute := provider.Routes[capability]
		if hasRoute && contains(provider.Capabilities, capability) {
			return provider, route, true
		}
	}
	return Provider{}, Route{}, false
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

var mappingPlaceholder = regexp.MustCompile(`\{\{\s*([^{}\s]+)\s*\}\}`)

func validateMapping(value any, input map[string]any) error {
	switch typed := value.(type) {
	case string:
		for _, match := range mappingPlaceholder.FindAllStringSubmatch(typed, -1) {
			if _, exists := input[match[1]]; !exists {
				return fmt.Errorf("request mapping references missing field %q", match[1])
			}
		}
	case map[string]any:
		for _, nested := range typed {
			if err := validateMapping(nested, input); err != nil {
				return err
			}
		}
	case []any:
		for _, nested := range typed {
			if err := validateMapping(nested, input); err != nil {
				return err
			}
		}
	}
	return nil
}

func resolveMapping(template map[string]any, input map[string]any) map[string]any {
	if len(template) == 0 {
		return input
	}
	resolved := make(map[string]any, len(template))
	for key, value := range template {
		resolved[key] = resolveValue(value, input)
	}
	return resolved
}

func resolveValue(value any, input map[string]any) any {
	switch typed := value.(type) {
	case string:
		if strings.HasPrefix(typed, "{{") && strings.HasSuffix(typed, "}}") {
			key := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(typed, "{{"), "}}"))
			if replacement, exists := input[key]; exists {
				return replacement
			}
		}
		result := typed
		for key, replacement := range input {
			result = strings.ReplaceAll(result, "{{"+key+"}}", fmt.Sprint(replacement))
		}
		return result
	case map[string]any:
		return resolveMapping(typed, input)
	case []any:
		resolved := make([]any, len(typed))
		for i := range typed {
			resolved[i] = resolveValue(typed[i], input)
		}
		return resolved
	default:
		return value
	}
}

func (gateway *Gateway) listCapabilities(response http.ResponseWriter) {
	seen := make(map[string]struct{})
	for _, provider := range gateway.config.Providers {
		if provider.Disabled || gateway.providerStatus(provider.ID) == "offline" {
			continue
		}
		for _, capability := range provider.Capabilities {
			if _, routable := provider.Routes[capability]; routable {
				seen[capability] = struct{}{}
			}
		}
	}
	capabilities := make([]string, 0, len(seen))
	for capability := range seen {
		capabilities = append(capabilities, capability)
	}
	sort.Strings(capabilities)
	writeJSON(response, http.StatusOK, map[string]any{"object": "list", "data": capabilities})
}

func (gateway *Gateway) listProviders(response http.ResponseWriter) {
	providers := make([]providerView, 0, len(gateway.config.Providers))
	for _, provider := range gateway.config.Providers {
		providers = append(providers, providerView{
			ID: provider.ID, Name: provider.Name, ProviderType: provider.ProviderType,
			Capabilities: append([]string(nil), provider.Capabilities...),
			Disabled:     provider.Disabled, Status: gateway.providerStatus(provider.ID),
		})
	}
	writeJSON(response, http.StatusOK, map[string]any{"object": "list", "data": providers})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
