package runtime

import "log"

var debug bool

// SetDebug set debug state
func SetDebug(b bool) {
	debug = b
}

// Log output log
func Log(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// Dbg output debug log, when debug switch on
func Dbg(format string, v ...interface{}) {
	if debug {
		log.Printf(format, v...)
	}
}
