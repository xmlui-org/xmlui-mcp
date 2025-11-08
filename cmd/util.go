package main

import (
	"fmt"
	"io"
	"log"
)

// fprintf is a wrapper around fmt.Fprintf that logs errors instead of returning them.
// This is used for non-critical output operations where error handling would clutter the code.
// Returns the number of bytes written.
func fprintf(w io.Writer, format string, a ...any) int {
	n, err := fmt.Fprintf(w, format, a...)
	if err != nil {
		log.Printf("Failed to print to %v; %v", w, err)
	}
	return n
}
