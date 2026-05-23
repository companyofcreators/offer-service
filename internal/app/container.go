package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"

	offerApp "github.com/companyofcreators/offer-service/internal/application/offer"
	"github.com/companyofcreators/offer-service/internal/config"
	offerDomain "github.com/companyofcreators/offer-service/internal/domain/offer"
	"github.com/companyofcreators/offer-service/internal/infrastructure/db"
	"github.com/companyofcreators/offer-service/internal/infrastructure/kafka"
	wsinfra "github.com/companyofcreators/offer-service/internal/infrastructure/ws"
	httpHandler "github.com/companyofcreators/offer-service/internal/interfaces/http"
	"github.com/companyofcreators/offer-service/internal/pkg"
	"github.com/companyofcreators/offer-service/pkg/header_auth"
)

// Container holds all application dependencies.
type Container struct {
	Config       *config.Config
	Logger       *slog.Logger
	Pool         *sqlx.DB
	Producer     *kafka.Producer
	HeaderSigner *header_auth.HeaderSigner
	Handler      *httpHandler.Handler
	WSHub        *wsinfra.Hub
}

// NewContainer initializes all application dependencies.
func NewContainer(ctx context.Context) (*Container, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	log, err := pkg.NewLogger(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Database
	poolCfg := db.DefaultPostgresConfig(cfg.DBDSN)
	pool, err := db.NewPostgresPool(ctx, poolCfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create database pool: %w", err)
	}

	// Kafka producer
	producer := kafka.NewProducer(cfg.KafkaBrokersList(), log)

	// Repositories
	offerRepo := db.NewOfferRepo(pool)
	eventRepo := db.NewNegotiationEventRepo(pool)

	// Order client for validating customer ownership
	headerSigner := header_auth.NewHeaderSigner(cfg.HeaderHMACKey)
	orderClient := NewOrderClient(cfg.OrderServiceURL, headerSigner, log)

	// WebSocket Hub (implements offerDomain.Broadcaster)
	wsHub := wsinfra.NewHub(log)

	// Domain service
	service := offerDomain.NewService(offerRepo, eventRepo, producer, orderClient, wsHub, log)

	// Application use cases
	sendOfferUC := offerApp.NewSendOfferUseCase(service)
	withdrawOfferUC := offerApp.NewWithdrawOfferUseCase(service)
	acceptOfferUC := offerApp.NewAcceptOfferUseCase(service)
	rejectOfferUC := offerApp.NewRejectOfferUseCase(service)
	counterOfferUC := offerApp.NewCounterOfferUseCase(service)

	// HTTP handler
	handler := httpHandler.NewHandler(
		sendOfferUC,
		withdrawOfferUC,
		acceptOfferUC,
		rejectOfferUC,
		counterOfferUC,
		service,
		log,
	)

	return &Container{
		Config:       cfg,
		Logger:       log,
		Pool:         pool,
		Producer:     producer,
		HeaderSigner: headerSigner,
		Handler:      handler,
		WSHub:        wsHub,
	}, nil
}

// Close gracefully shuts down all dependencies.
func (c *Container) Close() {
	if c.Producer != nil {
		if err := c.Producer.Close(); err != nil {
			c.Logger.ErrorContext(context.Background(), "failed to close kafka producer", "error", err.Error())
		}
	}
	if c.Pool != nil {
		c.Pool.Close()
	}
}
