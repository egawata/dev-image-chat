package main

import "log"

var debugEnabled bool

// InitLogger sets up the debug logging flag.
func InitLogger(debug bool) {
	debugEnabled = debug
}

// Debugf logs a message only when debug mode is enabled.
func Debugf(format string, args ...any) {
	if debugEnabled {
		log.Printf(format, args...)
	}
}
