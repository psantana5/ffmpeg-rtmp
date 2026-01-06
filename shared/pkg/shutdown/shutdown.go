package shutdown

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Manager handles graceful shutdown
type Manager struct {
	shutdownFuncs []func(context.Context) error
	mu            sync.Mutex
	timeout       time.Duration
	doneChan      chan struct{}
	once          sync.Once
}

// New creates a new shutdown manager
func New(timeout time.Duration) *Manager {
	return &Manager{
		shutdownFuncs: make([]func(context.Context) error, 0),
		timeout:       timeout,
		doneChan:      make(chan struct{}),
	}
}

// Register adds a shutdown function
// Functions are called in reverse order (LIFO)
func (m *Manager) Register(fn func(context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdownFuncs = append(m.shutdownFuncs, fn)
}

// Wait blocks until shutdown signal is received
func (m *Manager) Wait() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	
	sig := <-sigChan
	fmt.Printf("\nReceived signal: %v\n", sig)
	fmt.Println("Initiating graceful shutdown...")
	
	// Close done channel to notify waiting goroutines
	m.once.Do(func() {
		close(m.doneChan)
	})
}

// Done returns a channel that is closed when shutdown is initiated
func (m *Manager) Done() <-chan struct{} {
	return m.doneChan
}

// Shutdown executes all registered shutdown functions
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	
	// Execute shutdown functions in reverse order (LIFO)
	for i := len(m.shutdownFuncs) - 1; i >= 0; i-- {
		fn := m.shutdownFuncs[i]
		
		if err := fn(ctx); err != nil {
			fmt.Printf("Shutdown function %d error: %v\n", i, err)
		}
	}
	
	fmt.Println("Graceful shutdown complete")
}

// WaitWithContext blocks until shutdown signal or context cancellation
func (m *Manager) WaitWithContext(ctx context.Context) error {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	
	select {
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal: %v\n", sig)
		fmt.Println("Initiating graceful shutdown...")
		m.Shutdown()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Example: Common shutdown functions

// StopHTTPServer creates a shutdown function for http.Server
func StopHTTPServer(server interface{ Shutdown(context.Context) error }, name string) func(context.Context) error {
	return func(ctx context.Context) error {
		fmt.Printf("Stopping %s HTTP server...\n", name)
		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to stop %s server: %w", name, err)
		}
		fmt.Printf("%s HTTP server stopped\n", name)
		return nil
	}
}

// CloseResource creates a shutdown function for io.Closer
func CloseResource(closer interface{ Close() error }, name string) func(context.Context) error {
	return func(ctx context.Context) error {
		fmt.Printf("Closing %s...\n", name)
		if err := closer.Close(); err != nil {
			return fmt.Errorf("failed to close %s: %w", name, err)
		}
		fmt.Printf("%s closed\n", name)
		return nil
	}
}

// WaitForJobs creates a shutdown function that waits for jobs to complete
func WaitForJobs(checkFunc func() bool, pollInterval time.Duration, resourceName string) func(context.Context) error {
	return func(ctx context.Context) error {
		fmt.Printf("Waiting for %s to complete...\n", resourceName)
		
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		
		for {
			if checkFunc() {
				fmt.Printf("%s completed\n", resourceName)
				return nil
			}
			
			select {
			case <-ctx.Done():
				return fmt.Errorf("timeout waiting for %s: %w", resourceName, ctx.Err())
			case <-ticker.C:
				// Continue waiting
			}
		}
	}
}
