// Package logging provides a process-wide structured logger (zerolog),
// configured from the application Config singleton.
package logging

import (
	"io"
	"os"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"github.com/acme-corp/mcp-runtime/pkg/config"
	"github.com/acme-corp/mcp-runtime/pkg/telemetry"
)

var (
	logger zerolog.Logger
	once   sync.Once
)

// Init configures and returns the process-wide logger from cfg. Configuration
// happens once; later calls return the already-built logger.
func Init(cfg *config.Config) zerolog.Logger {
	once.Do(func() { logger = build(cfg) })
	return logger
}

// L returns the process-wide logger, initializing it from config.Get() if Init
// has not yet been called.
func L() zerolog.Logger {
	once.Do(func() { logger = build(config.Get()) })
	return logger
}

func build(cfg *config.Config) zerolog.Logger {
	return buildWithWriter(cfg, os.Stderr)
}

// buildWithWriter builds the logger over raw, wrapping it in a RedactingWriter so
// secrets are scrubbed from every line regardless of how a field was added
// (defense-in-depth, T080). Separated from build for testability.
func buildWithWriter(cfg *config.Config, raw io.Writer) zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.SetGlobalLevel(parseLevel(cfg.LogLevel))

	out := telemetry.RedactingWriter{W: raw}
	var base zerolog.Logger
	if strings.EqualFold(cfg.LogFormat, "console") {
		base = zerolog.New(zerolog.ConsoleWriter{Out: out}).With().Timestamp().Logger()
	} else {
		base = zerolog.New(out).With().Timestamp().Logger()
	}
	return base.With().Str("service", "mcp-gateway").Str("env", cfg.Env).Logger()
}

func parseLevel(s string) zerolog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return zerolog.DebugLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
