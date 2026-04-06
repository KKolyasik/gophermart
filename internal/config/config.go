package config

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config хранит настройки приложения из флагов и переменных окружения.
type Config struct {
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string
	SecretKey            string
	ShutdownTimeout      time.Duration
}

// NewConfig собирает конфигурацию из флагов и переменных окружения.
func NewConfig() (*Config, error) {
	cfg := &Config{
		ShutdownTimeout: 30 * time.Second,
	}

	if err := parseEnv(cfg); err != nil {
		return nil, err
	}
	if err := parseFlags(cfg); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout <= 0 {
		return nil, fmt.Errorf("shutdown timeout must be positive, got %s", cfg.ShutdownTimeout)
	}

	return cfg, nil
}

func parseEnv(cfg *Config) error {
	viper.AutomaticEnv()

	if runAddress := viper.GetString("RUN_ADDRESS"); runAddress != "" {
		cfg.RunAddress = runAddress
	}

	if databaseURI := viper.GetString("DATABASE_URI"); databaseURI != "" {
		cfg.DatabaseURI = databaseURI
	}

	if accrualSystemAddress := viper.GetString("ACCRUAL_SYSTEM_ADDRESS"); accrualSystemAddress != "" {
		cfg.AccrualSystemAddress = accrualSystemAddress
	}

	if secretKey := viper.GetString("SECRET_KEY"); secretKey != "" {
		cfg.SecretKey = secretKey
	} else {
		secretKey, err := generateSecretKey(16)
		if err != nil {
			return fmt.Errorf("generate secret key: %w", err)
		}
		cfg.SecretKey = secretKey
	}

	if shutdownTimeoutRaw := viper.GetString("SHUTDOWN_TIMEOUT"); shutdownTimeoutRaw != "" {
		shutdownTimeout, err := time.ParseDuration(shutdownTimeoutRaw)
		if err != nil {
			return fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
		}
		cfg.ShutdownTimeout = shutdownTimeout
	}

	return nil
}

func parseFlags(cfg *Config) error {
	pflag.String("a", "", "server address")
	pflag.String("d", "", "database uri")
	pflag.String("r", "", "accrual system address")
	pflag.DurationP("shutdown-timeout", "s", 30*time.Second, "graceful shutdown timeout")
	pflag.Parse()
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		return fmt.Errorf("bind flags: %w", err)
	}

	if pflag.CommandLine.Changed("a") {
		cfg.RunAddress = viper.GetString("a")
	}
	if pflag.CommandLine.Changed("d") {
		cfg.DatabaseURI = viper.GetString("d")
	}
	if pflag.CommandLine.Changed("r") {
		cfg.AccrualSystemAddress = viper.GetString("r")
	}
	if pflag.CommandLine.Changed("shutdown-timeout") {
		cfg.ShutdownTimeout = viper.GetDuration("shutdown-timeout")
	}
	return nil
}

func generateSecretKey(size int) (string, error) {
	key := make([]byte, size)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return string(key), nil
}
