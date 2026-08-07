//go:build !windows

package main

import (
	"fmt"
	"os"
)

func showStartupMessage(message string) {
	fmt.Fprintln(os.Stderr, message)
}
