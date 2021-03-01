package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/user"
	"sync"

	"github.com/vyzo/libp2p-flare-test/util"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"

	noise "github.com/libp2p/go-libp2p-noise"
	quic "github.com/libp2p/go-libp2p-quic-transport"
	tls "github.com/libp2p/go-libp2p-tls"
	tcp "github.com/libp2p/go-tcp-transport"

	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("flare")

func init() {
	identify.ClientVersion = "flarec/0.1"
	logging.SetLogLevel("flare", "DEBUG")
	logging.SetLogLevel("p2p-holepunch", "DEBUG")
	logging.SetLogLevel("p2p-circuit", "DEBUG")
}

func main() {
	idTCPPath := flag.String("idTCP", "identity-tcp", "identity key file path for TCP host")
	idUDPPath := flag.String("idUDP", "identity-udp", "identity key file path for UDP host")
	cfgPath := flag.String("config", "config.json", "json configuration file")
	enableTCP := flag.Bool("tcp", true, "enable TCP host")
	enableUDP := flag.Bool("udp", true, "enable UDP host")
	listPeers := flag.Bool("listPeers", false, "list peers and exit")
	eagerTest := flag.Bool("eagerTest", false, "eagerly try to hole punch with all known peers and exit")
	nickname := flag.String("nick", "", "nickname for peer; defaults to the current user login id")
	quiet := flag.Bool("quiet", false, "only log errors")
	flag.Parse()

	if *quiet {
		logging.SetLogLevel("*", "ERROR")
	}

	persistentIds := !*listPeers && !*eagerTest

	var cfg Config
	err := util.LoadConfig(*cfgPath, &cfg)
	if err != nil {
		fatalf("error loading config: %s", err)
	}

	nick := *nickname
	if nick == "" {
		user, err := user.Current()
		if err != nil {
			fatalf("error getting current user: %s", err)
		}
		nick = user.Username
	}

	var clients []*Client

	if *enableTCP {
		var privk crypto.PrivKey
		if persistentIds {
			privk, err = util.LoadIdentity(*idTCPPath)
		} else {
			privk, err = util.GenerateIdentity()
		}
		if err != nil {
			fatalf("error loading TCP identity: %s", err)
		}

		id, err := peer.IDFromPrivateKey(privk)
		if err != nil {
			fatalf("error extracing peer ID: %s", err)
		}

		tracer, err := NewTracer(&cfg, id, "TCP", nick)
		if err != nil {
			fatalf("error creating tracer: %s", err)
		}
		defer tracer.Close()

		cm := NewConnManager()
		defer cm.Close()

		var opts []libp2p.Option
		opts = append(opts,
			libp2p.Identity(privk),
			libp2p.NoTransports,
			libp2p.Security(noise.ID, noise.New),
			libp2p.Security(tls.ID, tls.New),
			libp2p.Transport(tcp.NewTCPTransport),
			libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
			libp2p.ConnectionManager(cm),
			libp2p.EnableRelay(),
			libp2p.EnableHolePunching(holepunch.WithTracer(tracer)),
			libp2p.ForceReachabilityPrivate(),
		)

		host, err := libp2p.New(context.Background(), opts...)
		if err != nil {
			fatalf("error constructing TCP host: %s", err)
		}

		client, err := NewClient(host, tracer, &cfg, "TCP", nick)
		if err != nil {
			fatalf("error creating client: %s", err)
		}
		clients = append(clients, client)
	}

	if *enableUDP {
		var privk crypto.PrivKey
		if persistentIds {
			privk, err = util.LoadIdentity(*idUDPPath)
		} else {
			privk, err = util.GenerateIdentity()
		}
		if err != nil {
			fatalf("error loading UDP identity: %s", err)
		}

		id, err := peer.IDFromPrivateKey(privk)
		if err != nil {
			fatalf("error extracing peer ID: %s", err)
		}

		tracer, err := NewTracer(&cfg, id, "UDP", nick)
		if err != nil {
			fatalf("error creating tracer: %s", err)
		}
		defer tracer.Close()

		cm := NewConnManager()
		defer cm.Close()

		var opts []libp2p.Option
		opts = append(opts,
			libp2p.Identity(privk),
			libp2p.Security(noise.ID, noise.New),
			libp2p.Security(tls.ID, tls.New),
			libp2p.NoTransports,
			libp2p.Transport(quic.NewTransport),
			libp2p.ListenAddrStrings("/ip4/0.0.0.0/udp/0/quic"),
			libp2p.ConnectionManager(cm),
			libp2p.EnableRelay(),
			libp2p.EnableHolePunching(holepunch.WithTracer(tracer)),
			libp2p.ForceReachabilityPrivate(),
		)

		host, err := libp2p.New(context.Background(), opts...)
		if err != nil {
			fatalf("error constructing UDP host: %s", err)
		}

		client, err := NewClient(host, tracer, &cfg, "UDP", nick)
		if err != nil {
			fatalf("error creating client: %s", err)
		}
		clients = append(clients, client)
	}

	if *listPeers {
		for _, c := range clients {
			peers, err := c.ListPeers()
			if err != nil {
				fatalf("error retrieving peers: %s", err)
			}

			fmt.Printf("%s peers:\n", c.Domain())
			for _, p := range peers {
				fmt.Printf("\t%s [%s]\n", p.Info.ID, p.Nick)
			}
		}

		return
	}

	if *eagerTest {
		for _, c := range clients {
			fmt.Printf("Eagerly testing with %s\n", c.Domain())
			peers, err := c.ListPeers()
			if err != nil {
				fatalf("error retrieving peers: %s", err)
			}

			for _, p := range peers {
				err := c.Connect(p)
				if err != nil {
					fmt.Printf("\t%s [%s]: %s\n", p.Info.ID, p.Nick, err)
				} else {
					fmt.Printf("\t%s [%s]: OK\n", p.Info.ID, p.Nick)
				}
			}
		}

		return
	}

	// background mode
	var wg sync.WaitGroup
	for _, c := range clients {
		fmt.Printf("I am %s for %s\n", c.ID(), c.Domain())
		fmt.Printf("Addresses: \n")
		for _, a := range c.Addrs() {
			fmt.Printf("\t%s\n", a)
		}
		wg.Add(1)
		go c.Background(&wg)
	}

	wg.Wait()
}

func fatalf(template string, args ...interface{}) {
	fmt.Printf(template+"\n", args...)
	os.Exit(1)
}
