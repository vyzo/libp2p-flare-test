package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/vyzo/libp2p-flare-test/util"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"

	noise "github.com/libp2p/go-libp2p-noise"
	quic "github.com/libp2p/go-libp2p-quic-transport"
	tls "github.com/libp2p/go-libp2p-tls"
	tcp "github.com/libp2p/go-tcp-transport"

	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

var log = logging.Logger("flare")

func init() {
	identify.ClientVersion = "flared/0.1"
	logging.SetLogLevel("p2p-holepunch", "DEBUG")
}

func main() {
	idPath := flag.String("-id", "identity", "identity key file path")
	cfgPath := flag.String("-config", "config.json", "json configuration file")
	flag.Parse()

	privk, err := util.LoadIdentity(*idPath)
	if err != nil {
		panic(err)
	}

	var cfg Config
	err = util.LoadConfig(*cfgPath, &cfg)
	if err != nil {
		panic(err)
	}

	var opts []libp2p.Option

	opts = append(opts,
		libp2p.Identity(privk),
		libp2p.DisableRelay(),
		libp2p.Security(noise.ID, noise.New),
		libp2p.Security(tls.ID, tls.New),
		libp2p.ListenAddrStrings(cfg.ListenAddrs...),
		libp2p.NoTransports,
		libp2p.Transport(quic.NewTransport),
		libp2p.Transport(tcp.NewTCPTransport),
	)

	if len(cfg.AnnounceAddrs) > 0 {
		var addrs []ma.Multiaddr
		for _, s := range cfg.AnnounceAddrs {
			a := ma.StringCast(s)
			addrs = append(addrs, a)
		}
		opts = append(opts,
			libp2p.AddrsFactory(func([]ma.Multiaddr) []ma.Multiaddr {
				return addrs
			}),
		)
	}

	ctx := context.Background()
	host, err := libp2p.New(ctx, opts...)
	if err != nil {
		panic(err)
	}

	_ = NewDaemon(host, &cfg)

	fmt.Printf("I am %s\n", host.ID())
	fmt.Printf("Public Addresses:\n")
	for _, addr := range host.Addrs() {
		if manet.IsPublicAddr(addr) {
			fmt.Printf("\t%s/p2p/%s\n", addr, host.ID())
		}
	}

	select {}
}
