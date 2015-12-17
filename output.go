package xlogsentry

import (
	"fmt"
	"time"
	"net/http"
	"runtime"
	"os"

	"github.com/getsentry/raven-go"
	"github.com/rs/xlog"
)

var (
	xlogSeverityMap = map[string]xlog.Level{
		"debug": xlog.LevelDebug,
		"info":  xlog.LevelInfo,
		"warn":  xlog.LevelWarn,
		"error": xlog.LevelError,
	}

	severityMap = map[xlog.Level]raven.Severity{
		xlog.LevelDebug: raven.DEBUG,
		xlog.LevelInfo:  raven.INFO,
		xlog.LevelWarn:  raven.WARNING,
		xlog.LevelError: raven.ERROR,
	}
)

// Output is a xlog to sentry output
type Output struct {
	Timeout                 time.Duration
	StacktraceConfiguration StackTraceConfiguration

	client                  *raven.Client
	host                    string
}

// StackTraceConfiguration allows for configuring stacktraces
type StackTraceConfiguration struct {
	// whether stacktraces should be enabled
	Enable        bool
	// the level at which to start capturing stacktraces
	Level         xlog.Level
	// how many stack frames to skip before stacktrace starts recording
	Skip          int
	// the number of lines to include around a stack frame for context
	Context       int
	// the prefixes that will be matched against the stack frame.
	// if the stack frame's package matches one of these prefixes
	// sentry will identify the stack frame as "in_app"
	InAppPrefixes []string
}


func NewSentryOutput(DSN string, tags map[string]string) *Output {
	client, _ := raven.NewClient(DSN, tags)
	return newOutput(client)
}

func NewSentryOutputWithClient(client *raven.Client) *Output {
	return newOutput(client)
}

func newOutput(client *raven.Client) *Output {
	hostname, _ := os.Hostname()
	return &Output{
		Timeout: 300 * time.Millisecond,
		StacktraceConfiguration: StackTraceConfiguration{
			Enable:        false,
			Level:         xlog.LevelError,
			Skip:          4,
			Context:       0,
			InAppPrefixes: nil,
		},
		client: client,
		host: hostname,
	}
}

func getAndDel(fields map[string]interface{}, key string) (string, bool) {
	var (
		ok bool
		v   interface{}
		val string
	)
	if v, ok = fields[key]; !ok {
		return "", false
	}

	if val, ok = v.(string); !ok {
		return "", false
	}
	delete(fields, key)
	return val, true
}

func getAndDelRequest(fields map[string]interface{}, key string) (*http.Request, bool) {
	var (
		ok bool
		v   interface{}
		req *http.Request
	)
	if v, ok = fields[key]; !ok {
		return nil, false
	}
	if req, ok = v.(*http.Request); !ok || req == nil {
		return nil, false
	}
	delete(fields, key)
	return req, true
}

// Write implements xlog.Output interface
func (o Output) Write(fields map[string]interface{}) error {
	level := xlogSeverityMap[fields[xlog.KeyLevel].(string)]

	packet := raven.NewPacket(fields[xlog.KeyMessage].(string))
	packet.Timestamp = raven.Timestamp(fields[xlog.KeyTime].(time.Time))
	packet.Level = severityMap[level]
	packet.Logger = "xlog"


	delete(fields, xlog.KeyMessage)
	delete(fields, xlog.KeyTime)
	delete(fields, xlog.KeyLevel)
	delete(fields, xlog.KeyFile)


	if serverName, ok := getAndDel(fields, "host"); ok {
		packet.ServerName = serverName
	} else if serverName, ok := getAndDel(fields, "server_name"); ok {
		packet.ServerName = serverName
	} else {
		packet.ServerName = o.host
	}
	if release, ok := getAndDel(fields, "release"); ok {
		packet.Release = release
	}
	if culprit, ok := getAndDel(fields, "culprit"); ok {
		packet.Culprit = culprit
	} else if role, ok := getAndDel(fields, "role"); ok {
		packet.Culprit = role
	}
	if req, ok := getAndDelRequest(fields, "http_request"); ok {
		packet.Interfaces = append(packet.Interfaces, raven.NewHttp(req))
	}

	fields["runtime.Version"] = runtime.Version()
	fields["runtime.NumCPU"] = runtime.NumCPU()
	fields["runtime.GOMAXPROCS"] = runtime.GOMAXPROCS(0)
	fields["runtime.NumGoroutine"] = runtime.NumGoroutine()

	stConfig := o.StacktraceConfiguration
	if stConfig.Enable && level <= stConfig.Level {
		currentStacktrace := raven.NewStacktrace(stConfig.Skip, stConfig.Context, stConfig.InAppPrefixes)
		packet.Interfaces = append(packet.Interfaces, currentStacktrace)
	}

	packet.Extra = map[string]interface{}(fields)

	_, errCh := o.client.Capture(packet, nil)

	timeout := o.Timeout
	if timeout != 0 {
		timeoutCh := time.After(timeout)
		select {
		case err := <-errCh:
			return err
		case <-timeoutCh:
			return fmt.Errorf("No response from sentry server in %s", timeout)
		}
	}

	return nil
}
