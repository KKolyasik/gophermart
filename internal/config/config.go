package config

import (
	"crypto/rand"
	"fmt"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config хранит адреса внешних систем и секрет приложения.
type Config struct {
	RunAddress           string
	DatabaseURI          string
	AccrualSystemAddress string
	SecretKey            string
}

// NewConfig собирает конфигурацию из флагов и переменных окружения.
func NewConfig() (*Config, error) {
	cfg := &Config{}

	if err := parseFlags(cfg); err != nil {
		return nil, err
	}
	parseEnv(cfg)

	return cfg, nil
}

func parseEnv(cfg *Config) {
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
		cfg.SecretKey = generateSecretKey(16)
	}
}

func parseFlags(cfg *Config) error {
	pflag.String("a", "", "server address")
	pflag.String("d", "", "database uri")
	pflag.String("r", "", "accrual system address")
	pflag.Parse()
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		return fmt.Errorf("bind flags: %w", err)
	}

	cfg.RunAddress = viper.GetString("a")
	cfg.DatabaseURI = viper.GetString("d")
	cfg.AccrualSystemAddress = viper.GetString("r")
	return nil
}

func generateSecretKey(size int) string {
	key := make([]byte, size)
	_, err := rand.Read(key)
	if err != nil {
		panic(err)
	}
	return string(key)
}
