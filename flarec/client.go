package main

import (
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"

	ma "github.com/multiformats/go-multiaddr"
)

type Client struct {
	host   host.Host
	domain string
}

type ClientInfo struct {
	Nick string
	Info peer.AddrInfo
}

func NewClient(h host.Host, cfg *Config, domain string) *Client {
	return nil
}

func (c *Client) Domain() string {
	return c.domain
}

func (c *Client) ID() peer.ID {
	return c.host.ID()
}

func (c *Client) Addrs() []ma.Multiaddr {
	return c.host.Addrs()
}

func (c *Client) ListPeers() ([]*ClientInfo, error) {
	return nil, nil
}

func (c *Client) Connect(ci *ClientInfo) error {
	return nil
}

func (c *Client) Background() {

}
