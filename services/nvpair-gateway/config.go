// SPDX-FileCopyrightText: Copyright (c) 2026 AI Unlock Innovations Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
)

type Config struct {
	Listen    string     `json:"listen"`
	Providers []Provider `json:"providers"`
}

type Provider struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	ProviderType string            `json:"providerType"`
	BaseURL      string            `json:"baseUrl"`
	HealthCheck  string            `json:"healthCheck,omitempty"`
	Capabilities []string          `json:"capabilities"`
	Headers      map[string]string `json:"headers,omitempty"`
	Routes       map[string]Route  `json:"routes,omitempty"`
	Disabled     bool              `json:"disabled,omitempty"`
}

type Route struct {
	Method         string         `json:"method"`
	Path           string         `json:"path"`
	RequestMapping map[string]any `json:"requestMapping,omitempty"`
	ResponseType   string         `json:"responseType,omitempty"`
	TimeoutSeconds int            `json:"timeoutSeconds,omitempty"`
}

func LoadConfig(reader io.Reader) (Config, error) {
	var config Config
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	if config.Listen == "" {
		config.Listen = "127.0.0.1:14322"
	}

	seen := make(map[string]struct{}, len(config.Providers))
	for i := range config.Providers {
		provider := &config.Providers[i]
		provider.ID = strings.TrimSpace(provider.ID)
		if provider.ID == "" {
			return Config{}, fmt.Errorf("provider %d: id is required", i)
		}
		if _, exists := seen[provider.ID]; exists {
			return Config{}, fmt.Errorf("duplicate provider id %q", provider.ID)
		}
		seen[provider.ID] = struct{}{}
		if provider.Name == "" {
			return Config{}, fmt.Errorf("provider %q: name is required", provider.ID)
		}
		if provider.ProviderType == "" {
			return Config{}, fmt.Errorf("provider %q: providerType is required", provider.ID)
		}
		parsed, err := url.Parse(provider.BaseURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return Config{}, fmt.Errorf("provider %q: baseUrl must be an absolute HTTP URL", provider.ID)
		}
		if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return Config{}, fmt.Errorf("provider %q: baseUrl must not contain credentials or a query", provider.ID)
		}
		if len(provider.Capabilities) == 0 {
			return Config{}, fmt.Errorf("provider %q: at least one capability is required", provider.ID)
		}
	}
	return config, nil
}
