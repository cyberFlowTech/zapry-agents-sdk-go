package agentsdk

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// Shutdowner describes a component that supports graceful shutdown.
type Shutdowner interface {
	Shutdown(ctx context.Context) error
}

// Stopper describes a component that can be stopped.
type Stopper interface {
	Stop()
}

// Disconnecter describes a component that can disconnect all sessions/resources.
type Disconnecter interface {
	DisconnectAll() error
}

// ShutdownFunc is a custom shutdown step.
type ShutdownFunc func(ctx context.Context) error

// SDKRuntime aggregates optional SDK subsystems and provides one-stop shutdown.
type SDKRuntime struct {
	AutoConversation   Shutdowner
	ProactiveScheduler Stopper
	MCPManager         Disconnecter
	MemoryStore        MemoryStore
	Closers            []io.Closer
	ShutdownHooks      []ShutdownFunc
}

// Shutdown performs best-effort graceful shutdown for all configured components.
func (r *SDKRuntime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var errs []error

	if r.ProactiveScheduler != nil {
		r.ProactiveScheduler.Stop()
	}
	if r.AutoConversation != nil {
		if err := r.AutoConversation.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown auto conversation: %w", err))
		}
	}
	if r.MCPManager != nil {
		if err := r.MCPManager.DisconnectAll(); err != nil {
			errs = append(errs, fmt.Errorf("disconnect mcp manager: %w", err))
		}
	}

	if r.MemoryStore != nil {
		if closer, ok := r.MemoryStore.(io.Closer); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, fmt.Errorf("close memory store: %w", err))
			}
		}
	}

	for _, closer := range r.Closers {
		if closer == nil {
			continue
		}
		if err := closer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close resource: %w", err))
		}
	}

	for _, hook := range r.ShutdownHooks {
		if hook == nil {
			continue
		}
		if err := hook(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown hook failed: %w", err))
		}
	}

	return errors.Join(errs...)
}
