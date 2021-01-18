package runtime

import (
	"fmt"
	"runtime"
	"strings"
)

// Trace stack trace
func Trace(prefix string) []string {
	var logs []string
	n := 1
	for {
		n++
		pc, file, line, ok := runtime.Caller(n)
		if !ok {
			break
		}
		f := runtime.FuncForPC(pc)
		name := f.Name()
		if strings.HasPrefix(name, "runtime.") {
			continue
		}
		logs = append(logs, fmt.Sprintf(prefix+"(%s:%d) %s", file, line, name))
	}
	return logs
}

func catch(err *error, handler ...func()) {
	if e := recover(); e != nil {
		*err = e.(error)
	}
	for _, h := range handler {
		h()
	}
}
