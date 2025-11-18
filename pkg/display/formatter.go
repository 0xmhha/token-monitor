package display

import (
	"fmt"
	"io"
)

// New creates a new formatter based on configuration.
//
// Parameters:
//   - cfg: Formatter configuration
//
// Returns a configured Formatter.
func New(cfg Config) Formatter {
	// Set defaults.
	if cfg.Format == "" {
		cfg.Format = FormatTable
	}

	switch cfg.Format {
	case FormatJSON:
		return &jsonFormatter{config: cfg}
	case FormatSimple:
		return &simpleFormatter{config: cfg}
	case FormatTable:
		fallthrough
	default:
		return &tableFormatter{config: cfg}
	}
}

// formatNumber formats a number with thousand separators.
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	// Convert to string and add commas.
	s := fmt.Sprintf("%d", n)
	result := ""
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result += ","
		}
		result += string(c)
	}
	return result
}

// formatFloat formats a float with specified precision.
func formatFloat(f float64, precision int) string {
	format := fmt.Sprintf("%%.%df", precision)
	return fmt.Sprintf(format, f)
}

// validateDimensions validates dimension names.
func validateDimensions(dimensions []string) error {
	if len(dimensions) == 0 {
		return fmt.Errorf("no dimensions specified")
	}
	return nil
}

// writeHeader writes a section header.
func writeHeader(w io.Writer, title string, compact bool) error {
	if compact {
		_, err := fmt.Fprintf(w, "%s\n", title)
		return err
	}

	separator := ""
	for i := 0; i < len(title); i++ {
		separator += "="
	}

	_, err := fmt.Fprintf(w, "\n%s\n%s\n\n", title, separator)
	return err
}
