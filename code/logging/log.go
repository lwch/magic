package logging

import (
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/lwch/runtime"
)

const (
	levelDebug int = iota
	levelInfo
	levelError
)

func init() {
	log.SetOutput(os.Stdout)
	rand.Seed(time.Now().UnixNano())
}

// Debug debug log
func Debug(fmt string, a ...interface{}) {
	currentLogger.rotate()
	if rand.Intn(1000) < 1 {
		currentLogger.write("[DEBUG]"+fmt, a...)
	}
}

// Info info log
func Info(fmt string, a ...interface{}) {
	currentLogger.rotate()
	currentLogger.write("[INFO]"+fmt, a...)
}

// Error error log
func Error(fmt string, a ...interface{}) {
	currentLogger.rotate()
	trace := strings.Join(runtime.Trace("  + "), "\n")
	currentLogger.write("[ERROR]"+fmt+"\n"+trace, a...)
}

// Flush flush log
func Flush() {
	currentLogger.flush()
}
