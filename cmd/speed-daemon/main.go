package main

import (
	"log"
	"net"
)

const port = ":8080"

// DATA TYPE
// type str is a length-prefixed byte slice
type str []byte

// MESSAGES
// each message begins with a sing u8
type Error struct {
	msg str
}
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
type IAmCamera struct {
	road  uint16
	mile  uint16
	limit uint16
}
type IAmDispatcher struct {
	numroads uint8
	roads    []uint16
}

// each camera provides is fields when it connects to the server
// cameras report each plate they observe, along with a timestamp
// timestamps are unix timestamps that are unisgned
type camera struct {
	road       uint16
	location   string
	speedLimit int
}

// when server detects car speeding at 2 points on the same road, it finds the corresponding dispatcher and has it deliver a ticket
type dispatcher struct {
	roads []uint16
}

// roads are identified as 0 - 65535
// roads have a single speed limit
// road positions are identified by # of miles from start of road

func handleConnection(conn net.Conn) {
	defer conn.Close()
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
