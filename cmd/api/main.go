package main

import (
	"context"
	"crypto/rsa"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/companyofcreators/offer-service/internal/app"
	httpHandler "github.com/companyofcreators/offer-service/internal/interfaces/http"
	wshandler "github.com/companyofcreators/offer-service/internal/interfaces/ws"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize application container
	container, err := app.NewContainer(ctx)
	if err != nil {
		panic("failed to initialize application: " + err.Error())
	}
	defer container.Close()

	log := container.Logger

	// Load JWT public key for WebSocket authentication
	jwtPublicKey, err := loadJWTPublicKey(container.Config.JWTPublicKeyPath)
	if err != nil {
		log.ErrorContext(ctx, "failed to load JWT public key", "error", err.Error())
		os.Exit(1)
	}

	// Create WebSocket handler
	wsHandler := wshandler.NewHandler(container.WSHub, jwtPublicKey, log, container.Config.WSAllowedOrigin)

	// Create HTTP router
	router := httpHandler.NewRouter(container.Handler, container.HeaderSigner, log, wsHandler.Upgrade)

	// Create HTTP server
	server := &http.Server{
		Addr:         container.Config.HTTPAddress,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.InfoContext(ctx, "starting offer service",
			"address", container.Config.HTTPAddress,
			"ws_endpoint", "ws://"+container.Config.HTTPAddress+"/ws",
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.ErrorContext(ctx, "server failed to start", "error", err.Error())
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.InfoContext(ctx, "shutting down offer service...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.ErrorContext(ctx, "server forced to shutdown", "error", err.Error())
	}

	log.InfoContext(ctx, "offer service stopped")
}

func loadJWTPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return jwt.ParseRSAPublicKeyFromPEM(data)
}
