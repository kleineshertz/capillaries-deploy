package l

import (
	"runtime"
	"strings"
)

func CurFuncName() string {
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return frame.Function[strings.LastIndex(frame.Function, "/")+1:]
}
