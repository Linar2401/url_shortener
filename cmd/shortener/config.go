package main

import (
	"errors"
	"flag"
	"strconv"
	"strings"
)

type NetAddress struct {
	Host string
	Port int
}

type DefaultConfig struct {
	ServeAddress  NetAddress
	ResultAddress NetAddress
}

func (a NetAddress) String() string {
	return a.Host + ":" + strconv.Itoa(a.Port)
}

func (a *NetAddress) Set(s string) error {
	hp := strings.Split(s, ":")
	if len(hp) != 2 {
		return errors.New("Need address in a form host:port")
	}
	port, err := strconv.Atoi(hp[1])
	if err != nil {
		return err
	}
	a.Host = hp[0]
	a.Port = port
	return nil
}

func parseFlags() DefaultConfig {
	flagRunAddr := new(NetAddress)
	flagRunAddr.Host = "localhost"
	flagRunAddr.Port = 8080
	_ = flag.Value(flagRunAddr)
	flag.Var(flagRunAddr, "a", "address and port to run server")

	flagResultAddr := new(NetAddress)
	flagResultAddr.Host = "localhost"
	flagResultAddr.Port = 8080
	_ = flag.Value(flagResultAddr)
	flag.Var(flagResultAddr, "b", "address and port to answer")

	flag.Parse()

	config := DefaultConfig{
		ServeAddress:  *flagRunAddr,
		ResultAddress: *flagResultAddr,
	}

	return config
}
