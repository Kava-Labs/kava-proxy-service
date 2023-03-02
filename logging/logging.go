package logging

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
)

// ServiceLogger is a json structured leveled logger
// used by the service to log messages to stdout
type ServiceLogger struct {
	*zerolog.Logger
}

var (
	serviceLogLevelToZeroLogLevel = map[string]zerolog.Level{
		"DEBUG": zerolog.DebugLevel,
		"INFO":  zerolog.InfoLevel,
		"ERROR": zerolog.ErrorLevel,
	}
)

// New creates and returns a new ServiceLogger and error (if any).
func New(logLevel string) (ServiceLogger, error) {
	zerologLevel, exists := serviceLogLevelToZeroLogLevel[logLevel]
	if !exists {
		return ServiceLogger{}, fmt.Errorf("invalid zero log level provided %s ", logLevel)
	}

	serviceLog := zerolog.New(os.Stdout).With().Timestamp().Caller().Logger()

	zerolog.SetGlobalLevel(zerologLevel)

	return ServiceLogger{
		Logger: &serviceLog,
	}, nil
}
