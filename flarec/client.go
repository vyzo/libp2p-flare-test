package main

import (
	"context"
	"fmt"
	"time"

	pb "github.com/vyzo/libp2p-flare-test/pb"
	"github.com/vyzo/libp2p-flare-test/proto"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/libp2p/go-msgio/protoio"
	ma "github.com/multiformats/go-multiaddr"
)

var bootstrappersTCPString = []string{
	"/ip4/147.75.83.83/tcp/4001/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
	"/ip4/147.75.77.187/tcp/4001/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
	"/ip4/147.75.94.115/tcp/4001/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
	"/ip4/147.75.109.213/tcp/4001/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
	"/ip4/147.75.109.29/tcp/4001/p2p/QmZa1sAxajnQjVM8WjWXoMbmPd7NsWhfKsPkErzpm9wGkp",
}

var bootstrappersUDPString = []string{
	"/ip4/147.75.83.83/udp/4001/quic/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
	"/ip4/147.75.77.187/udp/4001/quic/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
	"/ip4/147.75.94.115/udp/4001/quic/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
	"/ip4/147.75.109.213/udp/4001/quic/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
	"/ip4/147.75.109.29/udp/4001/quic/p2p/QmZa1sAxajnQjVM8WjWXoMbmPd7NsWhfKsPkErzpm9wGkp",
}

var bootstrappersTCP []*peer.AddrInfo
var bootstrappersUDP []*peer.AddrInfo

func init() {
	for _, a := range bootstrappersTCPString {
		pi, err := parseAddrInfo(a)
		if err != nil {
			panic(err)
		}

		bootstrappersTCP = append(bootstrappersTCP, pi)
	}

	for _, a := range bootstrappersUDPString {
		pi, err := parseAddrInfo(a)
		if err != nil {
			panic(err)
		}

		bootstrappersUDP = append(bootstrappersUDP, pi)
	}
}

type Client struct {
	host   host.Host
	cfg    *Config
	domain string
}

type ClientInfo struct {
	Nick string
	Info peer.AddrInfo
}

func NewClient(h host.Host, cfg *Config, domain string) *Client {
	return &Client{
		host:   h,
		cfg:    cfg,
		domain: domain,
	}
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
	s, err := c.connectToServer()
	if err != nil {
		return nil, fmt.Errorf("error connecting to flare server: %w", err)
	}
	defer s.Close()

	return c.getPeers(s)
}

func (c *Client) getPeers(s network.Stream) ([]*ClientInfo, error) {
	s.SetDeadline(time.Now().Add(time.Minute))

	var msg pb.FlareMessage
	wr := protoio.NewDelimitedWriter(s)
	rd := protoio.NewDelimitedReader(s, 1<<20)

	msg.Type = pb.FlareMessage_GETPEERS.Enum()
	msg.GetPeers = &pb.GetPeers{Domain: &c.domain}

	if err := wr.WriteMsg(&msg); err != nil {
		s.Reset()
		return nil, fmt.Errorf("error writing request to server: %w", err)
	}

	msg.Reset()

	if err := rd.ReadMsg(&msg); err != nil {
		s.Reset()
		return nil, fmt.Errorf("error reading server response: %w", err)
	}

	peerlist := msg.GetPeerList()
	if peerlist == nil {
		s.Reset()
		return nil, fmt.Errorf("bad server response: missing peer list")
	}

	result := make([]*ClientInfo, 0, len(peerlist.GetPeers()))
	for _, pi := range peerlist.GetPeers() {
		ci, err := peerInfoToClientInfo(pi)
		if err != nil {
			s.Reset()
			return nil, fmt.Errorf("error parsing client info: %w", err)
		}

		result = append(result, ci)
	}

	s.SetDeadline(time.Time{})

	return result, nil
}

func (c *Client) Connect(ci *ClientInfo) error {
	err := c.connectToBootstrappers()
	if err != nil {
		return fmt.Errorf("error connecting to bootstrappers: %w", err)
	}

	// let identify get our observed addresses before starting
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	err = c.host.Connect(ctx, ci.Info)
	if err != nil {
		return fmt.Errorf("error esstablishing initial connection to peer: %w", err)
	}

	deadline := time.After(time.Minute)
poll:
	for {
		for _, conn := range c.host.Network().ConnsToPeer(ci.Info.ID) {
			if !isRelayConn(conn) {
				return nil
			}
		}

		select {
		case <-deadline:
			break poll
		case <-time.After(time.Second):
		}
	}

	return fmt.Errorf("no direct connection to peer")
}

func (c *Client) connectToBootstrappers() error {
	var pis []*peer.AddrInfo
	if c.domain == "TCP" {
		pis = bootstrappersTCP
	} else {
		pis = bootstrappersUDP
	}

	for _, pi := range pis {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := c.host.Connect(ctx, *pi)
		cancel()

		if err != nil {
			return fmt.Errorf("error connecting to bootstrapper %s: %w", pi.ID, err)
		}
	}

	return nil
}

func (c *Client) Background() {

}

func (c *Client) connectToServer() (network.Stream, error) {
	pi, err := c.serverAddress()
	if err != nil {
		return nil, fmt.Errorf("error connecting to server: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	err = c.host.Connect(ctx, *pi)
	if err != nil {
		return nil, fmt.Errorf("error connecting to server: %w", err)
	}

	s, err := c.host.NewStream(ctx, proto.ProtoID)
	if err != nil {
		return nil, fmt.Errorf("error opening stream to server: %W", err)
	}

	// authenticate
	s.SetDeadline(time.Now().Add(time.Minute))

	var msg pb.FlareMessage
	wr := protoio.NewDelimitedWriter(s)
	rd := protoio.NewDelimitedReader(s, 4096)

	nonce, err := proto.Nonce()
	if err != nil {
		s.Reset()
		return nil, fmt.Errorf("error generating authen nonce: %w", err)
	}

	msg.Type = pb.FlareMessage_AUTHEN.Enum()
	msg.Authen = &pb.Authen{Nonce: nonce}

	if err := wr.WriteMsg(&msg); err != nil {
		s.Reset()
		return nil, fmt.Errorf("error writing authen message: %w", err)
	}

	msg.Reset()

	if err := rd.ReadMsg(&msg); err != nil {
		s.Reset()
		return nil, fmt.Errorf("error reading challenge message: %w", err)
	}

	if t := msg.GetType(); t != pb.FlareMessage_CHALLENGE {
		s.Reset()
		return nil, fmt.Errorf("unexpected server response: expected challenge, got %d", t)
	}

	challenge := msg.GetChallenge()
	if challenge == nil {
		s.Reset()
		return nil, fmt.Errorf("unexpected server response: missing challenge")
	}

	serverProof := challenge.GetProof()
	serverNonce := challenge.GetNonce()
	if !proto.Verify(c.cfg.Secret, nonce, serverProof) {
		s.Reset()
		return nil, fmt.Errorf("unexpected server response: authentication failure")
	}

	proof := proto.Proof(c.cfg.Secret, serverNonce)

	msg.Reset()
	msg.Type = pb.FlareMessage_RESPONSE.Enum()
	msg.Response = &pb.Response{Proof: proof}

	if err := wr.WriteMsg(&msg); err != nil {
		s.Reset()
		return nil, fmt.Errorf("error writing response to server: %w", err)
	}

	s.SetDeadline(time.Time{})

	return s, nil
}

func (c *Client) serverAddress() (*peer.AddrInfo, error) {
	if c.domain == "TCP" {
		return parseAddrInfo(c.cfg.ServerAddrTCP)
	} else {
		return parseAddrInfo(c.cfg.ServerAddrUDP)
	}
}

func peerInfoToClientInfo(pi *pb.PeerInfo) (*ClientInfo, error) {
	result := new(ClientInfo)
	result.Nick = pi.GetNick()

	pid, err := peer.IDFromBytes(pi.GetPeerID())
	if err != nil {
		return nil, fmt.Errorf("error parsing peer ID: %w", err)
	}
	result.Info.ID = pid

	for _, ab := range pi.GetAddrs() {
		a, err := ma.NewMultiaddrBytes(ab)
		if err != nil {
			return nil, fmt.Errorf("error parsing multiaddr: %w", err)
		}
		result.Info.Addrs = append(result.Info.Addrs, a)
	}

	return result, nil
}

func isRelayConn(conn network.Conn) bool {
	addr := conn.RemoteMultiaddr()
	_, err := addr.ValueForProtocol(ma.P_CIRCUIT)
	return err == nil
}

func parseAddrInfo(s string) (*peer.AddrInfo, error) {
	addr, err := ma.NewMultiaddr(s)
	if err != nil {
		return nil, fmt.Errorf("error parsing address: %w", err)
	}
	return peer.AddrInfoFromP2pAddr(addr)
}
