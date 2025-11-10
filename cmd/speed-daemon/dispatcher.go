package main

import (
	"net"
)

func newDispatcherFromConn(conn net.Conn, dispatcherEvents chan<- *event) error {
	panic("unimplemented")
}

func handleDispatcher(conn net.Conn, dispatcherEvents chan<- *event) {
	panic("unimplemented")
}

func dispatcherManager(dispatcherEvents chan *event) {
	panic("unimplemented")
}
