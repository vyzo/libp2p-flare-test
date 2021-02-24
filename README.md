# Project Flare Alpha Testing Programs

Project Flare aims to bring pervasive signaling infrastructure for
NAT traversal with hole punching in libp2p applications.  During Phase
0/1, we are alpha testing and collecting metrics using a fixed (limited) relay
and a presence service.  You can participate in alpha testing by
running the `flarec` binary, with a configuration supplied by the test
administrator.

Contact us at libp2p-at-libp2p.io if you want to participate in the ongoing alpha testing.

## Quick Start

Checkout the repo and run the `flarec` binary using the supplied configuration.
Please leave the program running in the background for a few days, so that we can collect
metrics about hole punching success.

To build:
```
$ git clone git@github.com:vyzo/libp2p-flare-test.git
$ cd libp2p-flare-test/cmd/flarec
$ go build
```

To run, first place the `config.json` file you received into the flarec directory and
the just run `flarec`:
```
$ cd libp2p-flare-test/cmd/flarec
$ cp /path/to/config.json .
$ ./flarec
```

### Command Line Options

The `flarec` program supports the following command line options:
```
 -idTCP <path>
  persistent identity key file path for TCP host; defaults to identity-tcp.
 -idUDP <path>
  persistent identity key file path for UDP host; defaults to identity-udp.
 -config <path>
  path to json configuration; defaults to config.json.
 -enableTCP[=false]
  enable (or disable) TCP host; enabled by default.
 -enableUDP[=false]
  enable (or disable) UDP host; enabled by default.
 -nick <nickname>
  nickname for your peer; defaults to user login id.
 -quiet
  reduce logging output to just ERRORs.
 -listPeers
  lists peers that have announced presence and exits
 -eaterTest
  eagerly try to connect to all peers that have announced presence
```

Running `flarec -listPeers` will list the current peers that have announced presence and exit.
Running `flarec -eagerTest` will fetch the current peers and attempt to connect with hole punching to all of them.

## Administering an Alpha test

If you want to run your own testing infrastructure and recruit your own users, you will need two things:
- A limited relay server; see [libp2p-relay](https://github.com/vyzo/libp2p-relay) for implementation.
- The `flared` daemon, available in this package.

Once you have those two daemons up and running, create a `config.json`
client configuration file (see `cmd/flarec/config.go`), distribute it to your
users, and you are ready to go!

## License

© vyzo; MIT License.
