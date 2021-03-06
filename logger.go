package logger

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// Type represents logger's type
type Type int

const (
	// Version is this package's version
	Version = "0.3.0"

	// CombineLoggerType is the standard Apache combined log output
	//
	// format:
	//
	// :remote-addr - :remote-user [:date[clf]] ":method :url
	// HTTP/:http-version" :status :res[content-length] ":referrer" ":user-agent"
	CombineLoggerType Type = iota
	// CommonLoggerType is the standard Apache common log output
	//
	// format:
	//
	// :remote-addr - :remote-user [:date[clf]] ":method :url
	// HTTP/:http-version" :status :res[content-length]
	CommonLoggerType

	//
	JsonLoggerType
	// DevLoggerType is useful for development
	//
	// format:
	//
	// :method :url :status :response-time ms - :res[content-length]
	DevLoggerType
	// ShortLoggerType is shorter than common, including response time
	//
	// format:
	//
	// :remote-addr :remote-user :method :url HTTP/:http-version :status
	// :res[content-length] - :response-time ms
	ShortLoggerType
	// TinyLoggerType is the minimal ouput
	//
	// format:
	//
	// :method :url :status :res[content-length] - :response-time ms
	TinyLoggerType

	timeFormat = "02/Jan/2006:15:04:05 -0700"
)

type responseLogger struct {
	rw     http.ResponseWriter
	start  time.Time
	status int
	size   int
}

func (rl *responseLogger) Header() http.Header {
	return rl.rw.Header()
}

func (rl *responseLogger) Write(bytes []byte) (int, error) {
	if rl.status == 0 {
		rl.status = http.StatusOK
	}

	size, err := rl.rw.Write(bytes)

	rl.size += size

	return size, err
}

func (rl *responseLogger) WriteHeader(status int) {
	rl.status = status

	rl.rw.WriteHeader(status)
}

func (rl *responseLogger) Flush() {
	f, ok := rl.rw.(http.Flusher)

	if ok {
		f.Flush()
	}
}

type loggerHanlder struct {
	h          http.Handler
	formatType Type
	writer     io.Writer
}

func (rh loggerHanlder) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	rl := &responseLogger{rw: res, start: time.Now()}

	log.SetFormatter(&log.JSONFormatter{})

	rh.h.ServeHTTP(rl, req)

	rh.write(rl, req)
}

func (rh loggerHanlder) write(rl *responseLogger, req *http.Request) {
	username := "-"

	if req.URL.User != nil {
		if name := req.URL.User.Username(); name != "" {
			username = name
		}
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}

	switch rh.formatType {
	case CombineLoggerType:
		fmt.Fprintln(rh.writer, strings.Join([]string{
			req.RemoteAddr,
			"-",
			username,
			"[" + rl.start.Format(timeFormat) + "]",
			`"` + req.Method,
			req.RequestURI,
			req.Proto + `"`,
			strconv.Itoa(rl.status),
			strconv.Itoa(rl.size),
			`"` + req.Referer() + `"`,
			`"` + req.UserAgent() + `"`,
		}, " "))
	case JsonLoggerType:
		log.WithFields(log.Fields{
			// request
			"request.host":       req.Host,
			"request.method":     req.Method,
			"request.proto":      req.Proto,
			"request.url":        req.URL,
			"request.referer":    req.Referer(),
			"request.user_agent": req.UserAgent(),
			"request.header":     req.Header,
			"start_time":         rl.start.Format(timeFormat),
			"body":               string(body),
			// response
			"response.status": strconv.Itoa(rl.status),
			"response.size":   strconv.Itoa(rl.size),
			"client_address":  req.RemoteAddr,
		}).Info("request processed")
	case CommonLoggerType:
		fmt.Fprintln(rh.writer, strings.Join([]string{
			req.RemoteAddr,
			"-",
			username,
			"[" + rl.start.Format(timeFormat) + "]",
			`"` + req.Method,
			req.RequestURI,
			req.Proto + `"`,
			strconv.Itoa(rl.status),
			strconv.Itoa(rl.size),
		}, " "))
	case DevLoggerType:
		fmt.Fprintln(rh.writer, strings.Join([]string{
			req.Method,
			req.RequestURI,
			strconv.Itoa(rl.status),
			parseResponseTime(rl.start),
			"-",
			strconv.Itoa(rl.size),
		}, " "))
	case ShortLoggerType:
		fmt.Fprintln(rh.writer, strings.Join([]string{
			req.RemoteAddr,
			username,
			req.Method,
			req.RequestURI,
			req.Proto,
			strconv.Itoa(rl.status),
			strconv.Itoa(rl.size),
			"-",
			parseResponseTime(rl.start),
		}, " "))
	case TinyLoggerType:
		fmt.Fprintln(rh.writer, strings.Join([]string{
			req.Method,
			req.RequestURI,
			strconv.Itoa(rl.status),
			strconv.Itoa(rl.size),
			"-",
			parseResponseTime(rl.start),
		}, " "))
	}
}

func parseResponseTime(start time.Time) string {
	return fmt.Sprintf("%.3f ms", time.Now().Sub(start).Seconds()/1e6)
}

// DefaultHandler returns a http.Handler that wraps h by using
// Apache combined log output and print to os.Stdout
func DefaultHandler(h http.Handler) http.Handler {
	return loggerHanlder{
		h:          h,
		formatType: CombineLoggerType,
		writer:     os.Stdout,
	}
}

// Handler returns a http.Hanlder that wraps h by using t type log output
// and print to writer
func Handler(h http.Handler, writer io.Writer, t Type) http.Handler {
	return loggerHanlder{
		h:          h,
		formatType: t,
		writer:     writer,
	}
}
