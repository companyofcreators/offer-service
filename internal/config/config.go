package config

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddress      string `env:"HTTP_ADDRESS" env-default:":8084"`
	DBDSN            string `env:"DB_DSN" env-required:"true"`
	KafkaBrokers     string `env:"KAFKA_BROKERS" env-default:"localhost:9092"`
	OrderServiceURL  string `env:"ORDER_SERVICE_URL" env-default:"http://localhost:8083"`
	UserServiceURL   string `env:"USER_SERVICE_URL" env-default:"http://localhost:8082"`
	HeaderHMACKey    string `env:"HEADER_HMAC_KEY" env-default:"diploma-internal-hmac-secret-key-2026"`
	JWTPublicKeyPath string `env:"JWT_PUBLIC_KEY_PATH" env-default:"../keys/public.pem"`
	WSAllowedOrigin  string `env:"WS_ALLOWED_ORIGIN" env-default:""`
	LogLevel         string `env:"LOG_LEVEL" env-default:"info"`
}

func Load() (*Config, error) {
	if err := godotenv.Load(".env"); err != nil {
		slog.Warn(".env file not found, using environment variables", "error", err)
	}

	var cfg Config
	if err := cleanenv.ReadConfig(".env", &cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.DBDSN == "" {
		return fmt.Errorf("DB_DSN is required")
	}
	if c.KafkaBrokers == "" {
		return fmt.Errorf("KAFKA_BROKERS is required")
	}
	if c.OrderServiceURL == "" {
		return fmt.Errorf("ORDER_SERVICE_URL is required")
	}
	return nil
}

func (c *Config) KafkaBrokersList() []string {
	if c.KafkaBrokers == "" {
		return []string{"localhost:9092"}
	}
	return splitBrokers(c.KafkaBrokers)
}

func splitBrokers(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
