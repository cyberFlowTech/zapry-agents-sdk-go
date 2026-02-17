package telegram

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

// SetupLogging configures the standard logger with timestamp format.
//
// If debug is true, log output includes file/line information.
// If logFile is non-empty, logs are written to both stdout and the specified file.
func SetupLogging(debug bool, logFile string) {
	flags := log.Ldate | log.Ltime | log.Lmsgprefix
	if debug {
		flags |= log.Lshortfile
	}

	var writers []io.Writer
	writers = append(writers, os.Stdout)

	if logFile != "" {
		// Ensure log directory exists
		dir := filepath.Dir(logFile)
		if dir != "" && dir != "." {
			os.MkdirAll(dir, 0o755)
		}

		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			log.Printf("[Logger] Failed to open log file %s: %v", logFile, err)
		} else {
			writers = append(writers, f)
			log.Printf("[Logger] Logging to file: %s", logFile)
		}
	}

	log.SetOutput(io.MultiWriter(writers...))
	log.SetFlags(flags)

	if debug {
		log.Println("[Logger] Debug mode enabled")
	}
}
