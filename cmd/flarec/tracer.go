package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"runtime"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"

	logzio "github.com/logzio/logzio-go"
)

type Tracer struct {
	logz   *logzio.LogzioSender
	id     peer.ID
	domain string
	nick   string
}

var _ holepunch.EventTracer = (*Tracer)(nil)

type Event struct {
	Time   int64 // UNIX time
	Domain string
	Peer   peer.ID
	Nick   string
	Type   string
	Evt    interface{}
}

const (
	AnnounceEvtT = "announce"
	ConnectEvtT  = "connect"
	TraceEvtT    = "trace"
)

type AnnounceEvt struct {
	OSType  string
	NATType string
}

type ConnectEvt struct {
	RemotePeer peer.ID
	RemoteNick string
	Success    bool
	Error      string `json:",omitempty"`
}

func NewTracer(cfg *Config, id peer.ID, domain, nick string) (*Tracer, error) {
	dir, err := ioutil.TempDir("", "flarec.*")
	if err != nil {
		return nil, err
	}

	logz, err := logzio.New(cfg.LogzioToken, logzio.SetTempDirectory(dir))
	if err != nil {
		return nil, err
	}

	return &Tracer{
		logz:   logz,
		id:     id,
		domain: domain,
		nick:   nick,
	}, nil
}

func (t *Tracer) send(et string, e interface{}) {
	evt := &Event{
		Time:   time.Now().Unix(),
		Domain: t.domain,
		Peer:   t.id,
		Nick:   t.nick,
		Type:   et,
		Evt:    e,
	}

	data, err := json.Marshal(evt)
	if err != nil {
		log.Errorf("error marshallng event: %s", err)
		return
	}

	err = t.logz.Send(data)
	if err != nil {
		log.Errorf("error shipping event to logz.io: %s", err)
	}
}

func (t *Tracer) Announce(natType string) {
	t.send(AnnounceEvtT, &AnnounceEvt{
		OSType:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		NATType: natType,
	})
}

func (t *Tracer) Connect(ci *ClientInfo, err error) {
	evt := &ConnectEvt{
		RemotePeer: ci.Info.ID,
		RemoteNick: ci.Nick,
		Success:    err == nil,
	}
	if err != nil {
		evt.Error = err.Error()
	}
	t.send(ConnectEvtT, evt)
}

func (t *Tracer) Trace(evt *holepunch.Event) {
	t.send(TraceEvtT, evt)
}

func (t *Tracer) Close() error {
	t.logz.Stop()
	return nil
}
