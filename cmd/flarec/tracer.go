package main

import (
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
)

type Tracer struct{}

var _ holepunch.EventTracer = (*Tracer)(nil)

func NewTracer(cfg *Config, id peer.ID, domain, nick string) *Tracer {
	return nil
}

func (t *Tracer) Trace(evt *holepunch.Event) {

}

func (t *Tracer) Close() error {
	return nil
}
