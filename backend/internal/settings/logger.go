package settings

import "log/slog"

type LoggerSettings struct {
	ResourceAttributes map[string]string
	LogLevel           slog.Level
}
