package log

import (
	"log"
	"os"
)

// Logger creates a standard logger configured for the agent.
func Logger() *log.Logger {
	logger := log.New(os.Stdout, "agent ", log.LstdFlags|log.Lshortfile)
	return logger
}
