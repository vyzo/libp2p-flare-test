package main

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	ma "github.com/multiformats/go-multiaddr"
)

var GracePeriod = 5 * time.Minute

type ConnManager struct {
	sync.Mutex

	ctx    context.Context
	cancel func()

	protected map[peer.ID]map[string]struct{}
	conns     map[peer.ID]map[network.Conn]time.Time
}

var _ connmgr.ConnManager = (*ConnManager)(nil)
var _ network.Notifiee = (*ConnManager)(nil)

func NewConnManager() *ConnManager {
	ctx, cancel := context.WithCancel(context.Background())
	c := &ConnManager{
		ctx:       ctx,
		cancel:    cancel,
		protected: make(map[peer.ID]map[string]struct{}),
		conns:     make(map[peer.ID]map[network.Conn]time.Time),
	}

	go c.background()

	return c
}

// ConnectionManager interface
func (c *ConnManager) TagPeer(peer.ID, string, int)                          {}
func (c *ConnManager) UntagPeer(p peer.ID, tag string)                       {}
func (c *ConnManager) UpsertTag(p peer.ID, tag string, upsert func(int) int) {}
func (c *ConnManager) GetTagInfo(p peer.ID) *connmgr.TagInfo                 { return nil }
func (c *ConnManager) TrimOpenConns(ctx context.Context)                     {}
func (c *ConnManager) Notifee() network.Notifiee                             { return c }

func (c *ConnManager) Protect(id peer.ID, tag string) {
	c.Lock()
	defer c.Unlock()

	tags, ok := c.protected[id]
	if !ok {
		tags = make(map[string]struct{})
		c.protected[id] = tags
	}

	_, protected := tags[tag]
	if protected {
		return
	}

	log.Debugf("protect peer %s for %s", id, tag)
	tags[tag] = struct{}{}
}

func (c *ConnManager) Unprotect(id peer.ID, tag string) bool {
	c.Lock()
	defer c.Unlock()

	log.Debugf("unprotect peer %s for %s", id, tag)

	tags, ok := c.protected[id]
	if !ok {
		return false
	}

	delete(tags, tag)
	if len(tags) > 0 {
		return true
	}

	delete(c.protected, id)
	return false
}

func (c *ConnManager) IsProtected(id peer.ID, tag string) bool {
	c.Lock()
	defer c.Unlock()

	_, protected := c.protected[id][tag]
	return protected
}

func (c *ConnManager) Close() error {
	c.cancel()
	return nil
}

// Notifee interface
func (c *ConnManager) Listen(network.Network, ma.Multiaddr)      {}
func (c *ConnManager) ListenClose(network.Network, ma.Multiaddr) {}

func (c *ConnManager) Connected(_ network.Network, conn network.Conn) {
	c.Lock()
	defer c.Unlock()

	p := conn.RemotePeer()
	conns, ok := c.conns[p]
	if !ok {
		conns = make(map[network.Conn]time.Time)
		c.conns[p] = conns
	}
	conns[conn] = time.Now()
}

func (c *ConnManager) Disconnected(_ network.Network, conn network.Conn) {
	c.Lock()
	defer c.Unlock()

	p := conn.RemotePeer()
	conns, ok := c.conns[p]
	if !ok {
		return
	}

	delete(conns, conn)
	if len(conns) > 0 {
		return
	}

	delete(c.conns, p)
}

func (c *ConnManager) OpenedStream(_ network.Network, _ network.Stream) {}
func (c *ConnManager) ClosedStream(_ network.Network, _ network.Stream) {}

// internal house keeping
func (c *ConnManager) background() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case now := <-ticker.C:
			c.trim(now)
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *ConnManager) trim(now time.Time) {
	c.Lock()
	defer c.Unlock()

	for p, conns := range c.conns {
		_, protected := c.protected[p]
		if protected {
			continue
		}

		for conn, opened := range conns {
			if len(conn.GetStreams()) > 0 {
				continue
			}

			if now.Sub(opened) < GracePeriod {
				continue
			}

			log.Debugf("closing stale network connection to %s {%s}", p, conn)

			err := conn.Close()
			if err != nil {
				log.Warnf("error closing connection to %s: %s", p, err)
			}

			delete(conns, conn)
		}

		if len(conns) == 0 {
			delete(c.conns, p)
		}
	}
}
