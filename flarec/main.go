package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/user"

	"github.com/vyzo/libp2p-flare-test/util"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"

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
		privk, err := util.LoadIdentity(*idTCPPath)
		if err != nil {
			fatalf("error loading TCP identity: %s", err)
		}

		var opts []libp2p.Option
		opts = append(opts,
			libp2p.Identity(privk),
			libp2p.EnableRelay(),
			libp2p.Security(noise.ID, noise.New),
			libp2p.Security(tls.ID, tls.New),
			libp2p.NoTransports,
			libp2p.Transport(tcp.NewTCPTransport),
			libp2p.EnableHolePunching(),
		)

		host, err := libp2p.New(context.Background(), opts...)
		if err != nil {
			fatalf("error constructing TCP host: %s", err)
		}

		client := NewClient(host, &cfg, "TCP", nick)
		clients = append(clients, client)
	}

	if *enableUDP {
		privk, err := util.LoadIdentity(*idUDPPath)
		if err != nil {
			fatalf("error loading UDP identity: %s", err)
		}

		var opts []libp2p.Option
		opts = append(opts,
			libp2p.Identity(privk),
			libp2p.EnableRelay(),
			libp2p.Security(noise.ID, noise.New),
			libp2p.Security(tls.ID, tls.New),
			libp2p.NoTransports,
			libp2p.Transport(quic.NewTransport),
			libp2p.EnableHolePunching(),
		)

		host, err := libp2p.New(context.Background(), opts...)
		if err != nil {
			fatalf("error constructing UDP host: %s", err)
		}

		client := NewClient(host, &cfg, "UDP", nick)
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
	for _, c := range clients {
		fmt.Printf("I am %s for %s\n", c.ID(), c.Domain())
		fmt.Printf("Addresses: \n")
		for _, a := range c.Addrs() {
			fmt.Printf("\t%s\n", a)
		}
		go c.Background()
	}

	if len(clients) == 0 {
		return
	}

	select {}
}

func fatalf(template string, args ...interface{}) {
	fmt.Printf(template+"\n", args...)
	os.Exit(1)
}
