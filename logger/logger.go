package logger

import (
	"bytes"
	"github.com/op/go-logging"
	"log"
	"strings"
	"time"
)

// ApiLogger defines extended logger with generic no-level logging option
type ApiLogger struct {
	log *log.Logger
	*logging.Logger
}

// New provides pre-configured Logger with stderr output and leveled filtering.
// Modules are not supported at the moment, but may be added in the future to make the logging setup more granular.
func New() Logger {

	// assign the backend and return the new logger
	l := logging.MustGetLogger("Generator")

	// collect HTTP errors into the regular log file
	buff := new(bytes.Buffer)
	al := ApiLogger{
		log:    log.New(buff, "http", 0),
		Logger: l,
	}

	// handle buffer logging
	go dryBuffer(buff, al.Logger)
	return &al
}

// dryBuffer collects log records from the buffer and re-send subset to the regular logger.
func dryBuffer(buf *bytes.Buffer, lg *logging.Logger) {
	builder := new(strings.Builder)
	for {
		time.Sleep(10 * time.Millisecond)
		for buf.Len() > 0 {
			// read all up to LF, which we discard
			line, err := buf.ReadBytes(0xa)
			line = line[:len(line)-1]

			// remove possible CR character from the end
			if line[len(line)-1] == 0xd {
				line = line[:len(line)-1]
			}

			// add the line to the builder
			builder.Write(line)

			// the LF was reached?
			if err == nil {
				lg.Error(builder.String())
				builder.Reset()
			}
		}
		buf.Reset()
	}
}

// ModuleName returns the name of the logger module.
func (al *ApiLogger) ModuleName() string {
	return al.Module
}

// Printf implements default non-leveled output.
// We assume the information is low in importance if passed to this function, so we relay it to Debug level.
func (al *ApiLogger) Printf(format string, args ...interface{}) {
	al.Debugf(format, args...)
}

// ModuleLogger derives new logger for sub-module.
func (al *ApiLogger) ModuleLogger(mod string) Logger {
	var sb strings.Builder
	sb.WriteString(al.Module)
	sb.WriteString(".")
	sb.WriteString(mod)

	l := logging.MustGetLogger(sb.String())
	return &ApiLogger{Logger: l, log: al.log}
}

// Log returns log.Logger compatible logging instance.
func (al *ApiLogger) Log() *log.Logger {
	return al.log
}
