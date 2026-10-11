package config

import "log/slog"

type ApplicationConfig struct {
	Service     string
	Version     string
	Environment string
	LogLevel    slog.Level
}
