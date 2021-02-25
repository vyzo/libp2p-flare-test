package main

type Config struct {
	Secret        string
	ServerAddrTCP string
	ServerAddrUDP string
	RelayAddrTCP  string
	RelayAddrUDP  string
	LogzioToken   string
}
