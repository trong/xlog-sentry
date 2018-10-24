package xlogsentry

import (
	"github.com/getsentry/raven-go"
	"github.com/rs/xlog"
	"strings"
	"testing"
	"time"
)

type dummyTransportArgs struct {
	url        string
	authHeader string
	packet     *raven.Packet
}

type dummyTransport struct {
	roundtripChan chan<- dummyTransportArgs
}

func (dtx *dummyTransport) Send(url, authHeader string, packet *raven.Packet) error {
	dtx.roundtripChan <- dummyTransportArgs{url, authHeader, packet}
	return nil
}

func dummyClient(roundtripChan chan<- dummyTransportArgs) *raven.Client {
	client, err := raven.New("http://secret@localhost/id")
	if err != nil {
		panic(err)
	}
	client.Transport = &dummyTransport{roundtripChan}
	return client
}

func TestWriteBasic(t *testing.T) {
	fields := map[string]interface{}{
		xlog.KeyMessage: "message",
		xlog.KeyTime:    time.Date(2017, time.January, 1, 0, 0, 0, 0, time.UTC),
		xlog.KeyLevel:   "info",
		xlog.KeyFile:    "file",
	}
	roundtripChan := make(chan dummyTransportArgs, 1)
	target := Output{
		Timeout: 1 * time.Second,
		StacktraceConfiguration: StackTraceConfiguration{
			true,
			xlog.LevelInfo,
			0,
			5,
			[]string{},
		},
		client: dummyClient(roundtripChan),
		host:   "host",
	}
	target.Write(fields)
	args, ok := <-roundtripChan
	if !ok {
		t.Log("no data has been sent")
		t.Fail()
	}
	if "http://localhost/api/id/store/" != args.url {
		t.Logf(`%+v != args.url (got %+v)`, "http://localhost/api/id/store/", args.url)
		t.Fail()
	}
	if strings.Index(args.authHeader, "secret") < 0 {
		t.Logf(`%+v not in args.authHeader (got %+v)`, "secret", args.authHeader)
		t.Fail()
	}
	if "2017-01-01T00:00:00Z" != time.Time(args.packet.Timestamp).Format(time.RFC3339) {
		t.Logf(`%+v != args.packet.Timestamp (got %+v)`, "2017-01-01T00:00:00Z", args.packet.Timestamp.Format(time.RFC3339))
		t.Fail()
	}
	if raven.INFO != args.packet.Level {
		t.Logf(`%+v != args.packet.Level (got %+v)`, "raven.INFO", args.packet.Level)
		t.Fail()
	}
	for _, k := range []string{xlog.KeyMessage, xlog.KeyTime, xlog.KeyLevel, xlog.KeyFile} {
		if _, ok := args.packet.Extra[k]; ok {
			t.Logf(`key %+v exists in args.packet.Extra`, k)
			t.Fail()
		}
		if _, ok := fields[k]; !ok {
			t.Logf(`key %+v does not exist in fields`, k)
			t.Fail()
		}
	}
}
