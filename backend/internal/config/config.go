package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Environment string
type Role string

const (
	Development Environment = "development"
	Production  Environment = "production"
	RoleAPI     Role        = "api"
	RoleWorker  Role        = "worker"
)

type Secret string

func (s Secret) Reveal() string { return string(s) }

func (Secret) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }

type Config struct {
	Environment       Environment
	LogLevel          slog.Level
	Role              Role
	Port              int
	DatabaseURL       Secret
	EncryptionKey     Secret
	WorkerCount       int
	QueueCapacity     int
	WorkerMaxAttempts int
	ShutdownTimeout   time.Duration
}

type Lookup func(string) string

func Load(lookup Lookup) (Config, error) {
	databaseURL, err := requiredSecret(lookup, "DATABASE_URL")
	if err != nil {
		return Config{}, err
	}
	encryptionKey, err := requiredSecret(lookup, "EVICTOR_ENCRYPTION_KEY")
	if err != nil {
		return Config{}, err
	}
	port, err := positiveInt(lookup, "PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	workers, err := positiveInt(lookup, "WORKER_COUNT", 4)
	if err != nil {
		return Config{}, err
	}
	capacity, err := positiveInt(lookup, "WORKER_QUEUE_CAPACITY", 100)
	if err != nil {
		return Config{}, err
	}
	maxAttempts, err := positiveInt(lookup, "WORKER_MAX_ATTEMPTS", 3)
	if err != nil {
		return Config{}, err
	}

	environment := Environment(defaultValue(lookup("EVICTOR_ENV"), string(Development)))
	if environment != Development && environment != Production {
		return Config{}, fmt.Errorf("EVICTOR_ENV must be %q or %q", Development, Production)
	}
	logLevel, err := parseLogLevel(defaultValue(lookup("LOG_LEVEL"), "info"))
	if err != nil {
		return Config{}, err
	}
	role, err := ParseRole(defaultValue(lookup("EVICTOR_ROLE"), string(RoleAPI)))
	if err != nil {
		return Config{}, err
	}
	return Config{
		Environment:       environment,
		LogLevel:          logLevel,
		Role:              role,
		Port:              port,
		DatabaseURL:       databaseURL,
		EncryptionKey:     encryptionKey,
		WorkerCount:       workers,
		QueueCapacity:     capacity,
		WorkerMaxAttempts: maxAttempts,
		ShutdownTimeout:   15 * time.Second,
	}, nil
}

func FromEnvironment() (Config, error) { return Load(os.Getenv) }

func ParseRole(value string) (Role, error) {
	role := Role(strings.TrimSpace(value))
	if role != RoleAPI && role != RoleWorker {
		return "", fmt.Errorf("EVICTOR_ROLE must be %q or %q", RoleAPI, RoleWorker)
	}
	return role, nil
}

func parseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(value) {
	case "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, errors.New("LOG_LEVEL must be debug, info, warn, or error")
	}
}

func requiredSecret(lookup Lookup, name string) (Secret, error) {
	if value := strings.TrimSpace(lookup(name)); value != "" {
		return Secret(value), nil
	}
	if path := strings.TrimSpace(lookup(name + "_FILE")); path != "" {
		value, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s_FILE: %w", name, err)
		}
		if value := strings.TrimSpace(string(value)); value != "" {
			return Secret(value), nil
		}
	}
	return "", errors.New("missing required environment variable " + name + " or " + name + "_FILE")
}

func positiveInt(lookup Lookup, name string, fallback int) (int, error) {
	raw := strings.TrimSpace(lookup(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}

func defaultValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
