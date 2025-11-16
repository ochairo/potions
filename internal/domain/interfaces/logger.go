// Package interfaces defines core domain contracts.
//
//nolint:revive // Package name 'interfaces' is intentional for domain layer
package interfaces

// Logger defines the interface for structured logging
type Logger interface {
	// Debug logs debug-level messages
	Debug(msg string, fields ...Field)

	// Info logs informational messages
	Info(msg string, fields ...Field)

	// Warn logs warning messages
	Warn(msg string, fields ...Field)

	// Error logs error messages
	Error(msg string, fields ...Field)
}

// Field represents a structured log field
type Field struct {
	Key   string
	Value interface{}
}

// F creates a new Field (convenience function)
func F(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// NoOpLogger is a logger that does nothing (useful for tests)
type NoOpLogger struct{}

// Debug does nothing (no-op implementation)
func (n *NoOpLogger) Debug(_ string, _ ...Field) {}

// Info does nothing (no-op implementation)
func (n *NoOpLogger) Info(_ string, _ ...Field) {}

// Warn does nothing (no-op implementation)
func (n *NoOpLogger) Warn(_ string, _ ...Field) {}

// Error does nothing (no-op implementation)
func (n *NoOpLogger) Error(_ string, _ ...Field) {}

// StdoutLogger logs to stdout (simple implementation)
type StdoutLogger struct{}

// Debug logs debug-level messages to stdout
func (s *StdoutLogger) Debug(msg string, fields ...Field) {
	s.log("DEBUG", msg, fields)
}

// Info logs informational messages to stdout
func (s *StdoutLogger) Info(msg string, fields ...Field) {
	s.log("INFO", msg, fields)
}

// Warn logs warning messages to stdout
func (s *StdoutLogger) Warn(msg string, fields ...Field) {
	s.log("WARN", msg, fields)
}

func (s *StdoutLogger) Error(msg string, fields ...Field) {
	s.log("ERROR", msg, fields)
}

func (s *StdoutLogger) log(level, msg string, fields []Field) {
	// Simple stdout logger for backward compatibility
	if len(fields) == 0 {
		println(level + ": " + msg)
	} else {
		print(level + ": " + msg)
		for _, f := range fields {
			print(" " + f.Key + "=")
			print(f.Value)
		}
		println()
	}
}
