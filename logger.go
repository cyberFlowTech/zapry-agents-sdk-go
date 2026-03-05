package agentsdk

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

// Logger is the SDK-wide logging abstraction.
// Callers can inject their own implementation via SetLogger.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// SlogLogger adapts slog.Logger to Logger.
type SlogLogger struct {
	logger *slog.Logger
}

// NewSlogLogger wraps a slog.Logger as SDK Logger.
func NewSlogLogger(logger *slog.Logger) *SlogLogger {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return &SlogLogger{logger: logger}
}

func (l *SlogLogger) Debug(msg string, args ...any) { l.logger.Debug(msg, args...) }
func (l *SlogLogger) Info(msg string, args ...any)  { l.logger.Info(msg, args...) }
func (l *SlogLogger) Warn(msg string, args ...any)  { l.logger.Warn(msg, args...) }
func (l *SlogLogger) Error(msg string, args ...any) { l.logger.Error(msg, args...) }

var (
	loggerMu      sync.RWMutex
	packageLogger Logger = NewSlogLogger(nil)
)

// SetLogger configures SDK global logger.
func SetLogger(logger Logger) error {
	if logger == nil {
		return errors.New("logger is nil")
	}
	loggerMu.Lock()
	defer loggerMu.Unlock()
	packageLogger = logger
	return nil
}

// GetLogger returns the current SDK logger.
func GetLogger() Logger {
	loggerMu.RLock()
	defer loggerMu.RUnlock()
	return packageLogger
}

func logDebugf(format string, args ...any) { GetLogger().Debug(fmt.Sprintf(format, args...)) }
func logInfof(format string, args ...any)  { GetLogger().Info(fmt.Sprintf(format, args...)) }
func logWarnf(format string, args ...any)  { GetLogger().Warn(fmt.Sprintf(format, args...)) }
func logErrorf(format string, args ...any) { GetLogger().Error(fmt.Sprintf(format, args...)) }
