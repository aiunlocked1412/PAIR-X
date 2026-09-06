// SPDX-FileCopyrightText: Copyright (c) 2026 AI Unlock Innovations Co., Ltd.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var Version = "dev"

func main() {
	configPath := flag.String("config", "providers.json", "path to the PAIR-X provider configuration")
	showVersion := flag.Bool("version", false, "print version and exit")
	healthInterval := flag.Duration("health-interval", 10*time.Second, "provider health-check interval")
	flag.Parse()

	if *showVersion {
		fmt.Println(Version)
		return
	}

	file, err := os.Open(*configPath)
	if err != nil {
		log.Fatalf("open config: %v", err)
	}
	config, err := LoadConfig(file)
	file.Close()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	gateway := NewGateway(config, &http.Client{Timeout: 30 * time.Second})
	gateway.RefreshHealth(ctx)
	go monitorHealth(ctx, gateway, *healthInterval)

	server := &http.Server{
		Addr:              config.Listen,
		Handler:           gateway,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("PAIR-X gateway %s listening on %s", Version, config.Listen)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("gateway server: %v", err)
	}
}

func monitorHealth(ctx context.Context, gateway *Gateway, interval time.Duration) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			gateway.RefreshHealth(ctx)
		}
	}
}
