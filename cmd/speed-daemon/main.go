package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
)

const port = ":8080"

// DATA TYPE
// type str is a length-prefixed byte slice
// 0-255 length ASCII char codes
type str []byte

// MESSAGES
// each message begins with a sing u8
type Error struct {
	msg str
}

func NewError(conn net.Conn) {}

type Plate struct {
	plate     str
	timestamp uint32
}
type Ticket struct {
	plate      str
	road       uint16
	mile1      uint16
	timestamp1 uint32
	mile2      uint16
	timestamp2 uint32
	speed      uint16 // 100x mph
}
type WantHeartbeat struct {
	interval uint32
}
type Heartbeat struct{}

// cameras report each plate they observe, along with a timestamp
// timestamps are unix timestamps that are unisgned
type IAmCamera struct {
	road  uint16
	mile  uint16
	limit uint16
}

// when server detects car speeding at 2 points on the same road, it finds the corresponding dispatcher and has it deliver a ticket
type IAmDispatcher struct {
	numroads uint8
	roads    []uint16
}

func handleCamera(conn net.Conn, signal chan struct{}) {
	defer func() {
		signal <- struct{}{}
	}()

	buf := make([]byte, 6)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		log.Printf("failed to read camera metadata from conn: %w", err)
		return
	}

	c := IAmCamera{
		road:  binary.BigEndian.Uint16(buf[0:2]),
		mile:  binary.BigEndian.Uint16(buf[2:4]),
		limit: binary.BigEndian.Uint16(buf[4:6]),
	}

	// add camera to a map of cameras via channel
	for {
		// Go do camera things
	}
}

func handlerDispatcher(conn net.Conn, signal chan struct{}) {
	defer func() {
		signal <- struct{}{}
	}()
}

// roads are identified as 0 - 65535
// roads have a single speed limit
// road positions are identified by # of miles from start of road

func handleConnection(conn net.Conn) {
	defer conn.Close()

	r := bufio.NewReader(conn)
	b, err := r.ReadByte()
	if err != nil {
		log.Printf("failed to read msgType: %s", err)
		return
	}

	signal := make(chan struct{})
	switch b {
	case 0x80:
		go handleCamera(conn, signal)
		<-signal
	case 0x81:
		go handlerDispatcher(conn, signal)
		<-signal
	default:
		log.Printf("recieved invalid clientType: 0x%x", b)
	}
}

func main() {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", port, err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
		}

		go handleConnection(conn)
	}
}
