package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"slices"
)

var dispatchers map[net.Conn]*IAmDispatcher

func newDispatcherFromConn(conn net.Conn, dispatcherEvents chan<- *event) error {
	var m IAmDispatcher
	if err := m.decode(conn); err != nil {
		return err
	}

	dispatcherEvents <- &event{conn: conn, msg: &m}
	return nil
}

func handleDispatcher(conn net.Conn, dispatcherEvents chan<- *event) {
	defer func() {
		dispatcherEvents <- &event{
			conn:   conn,
			msg:    nil,
			signal: make(chan struct{}),
		}
	}()

	var typ uint8
	for {
		err := binary.Read(conn, binary.BigEndian, &typ)
		if err != nil {
			log.Printf("failed to read message type from client %v: %e", conn, err)
			return
		}

		switch typ {

		case uint8(MsgTicket):
			var t Ticket
			if err := t.decode(conn); err != nil {
				log.Printf("0x%x: failed to decode: %v", typ, err)
				return
			}
			dispatcherEvents <- &event{conn: conn, msg: &t}
		default:
			sendErrorAndDisconnect(conn, fmt.Sprintf("Client: %v\nError: it is an error for a client to send the server a message with message type: 0x%x", conn, typ))
			return
		}
	}
}

func dispatcherManager(dispatcherEvents chan *event) {
	for e := range dispatcherEvents {
		switch msg := e.msg.(type) {
		case *IAmDispatcher:
			dispatchers[e.conn] = msg
		case *Ticket:
			for conn, dispatcher := range dispatchers {
				if slices.Contains(dispatcher.roads, msg.road) {
					if err := msg.encode(conn); err != nil {
						log.Printf("failed to send ticket to client %v: %e", conn, err)
					}
					break
				}
			}
		case *ClientDisconnect:
			delete(dispatchers, e.conn)
		}
	}
}
