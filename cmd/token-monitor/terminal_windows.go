//go:build windows

package main

// enableOutputProcessing is a no-op on Windows.
// Windows terminals do not use OPOST for newline conversion.
func enableOutputProcessing(_ int) {}
