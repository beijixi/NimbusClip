package log

import (
	"log"
	"os"
)

// Logger creates a standard logger for server components.
func Logger() *log.Logger {
	return log.New(os.Stdout, "server ", log.LstdFlags|log.Lshortfile)
}
