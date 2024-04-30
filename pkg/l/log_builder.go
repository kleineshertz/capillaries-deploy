package l

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type LogBuilder struct {
	Sb        *strings.Builder
	IsVerbose bool
	Header    string
	StartTs   time.Time
}

type LogMsg string

const LogColorReset string = "\033[0m"
const LogColorRed string = "\033[31m"
const LogColorGreen string = "\033[32m"

func NewLogBuilder(header string, isVerbose bool) *LogBuilder {
	lb := LogBuilder{Sb: &strings.Builder{}, IsVerbose: isVerbose, Header: header, StartTs: time.Now()}
	if lb.IsVerbose {
		lb.Sb.WriteString("\n===============================================\n")
		lb.Sb.WriteString(fmt.Sprintf("%s : started\n", lb.Header))
	} else {
		lb.Sb.WriteString(fmt.Sprintf("%s : ", lb.Header))
	}
	return &lb
}

func AddLogMsg(sb *strings.Builder, logMsg LogMsg) {
	sb.WriteString(string(logMsg))
}

func (lb *LogBuilder) AddObject(content string, o any) {
	if !lb.IsVerbose {
		return
	}
	lb.Sb.WriteString(fmt.Sprintf("%s\n", content))
	if o == nil {
		lb.Sb.WriteString("nil")
		return
	}
	b, err := json.Marshal(o)
	if err == nil {
		lb.Sb.WriteString(string(b) + "\n")
	} else {
		lb.Sb.WriteString(fmt.Sprintf("cannot marshal %T: %s", o, err.Error()))
	}
}

func (lb *LogBuilder) Add(content string) {
	if !lb.IsVerbose {
		return
	}
	lb.Sb.WriteString(fmt.Sprintf("%s\n", content))
}

func (lb *LogBuilder) Complete(err error) (LogMsg, error) {
	if lb.IsVerbose {
		lb.Sb.WriteString(fmt.Sprintf("%s : ", lb.Header))
	}
	lb.Sb.WriteString(fmt.Sprintf("elapsed %.3fs, ", time.Since(lb.StartTs).Seconds()))
	if err == nil {
		lb.Sb.WriteString(LogColorGreen + "OK" + LogColorReset)
	} else {
		lb.Sb.WriteString(LogColorRed + err.Error() + LogColorReset)
	}
	lb.Sb.WriteString("\n")
	return LogMsg(lb.Sb.String()), err
}
