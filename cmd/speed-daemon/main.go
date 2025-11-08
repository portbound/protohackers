package main

import (
	"bufio"
	"encoding/binary"
	"io"
	"log"
	"net"
)

const port = ":8080"

// roads are identified as 0 - 65535
// roads have a single speed limit
// road positions are identified by # of miles from start of road

// DATA TYPE
// type str is a length-prefixed byte slice
// 0-255 length ASCII char codes
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

type Camera struct {
	conn net.Conn
	data IAmCamera
}

type Dispatcher struct {
	conn net.Conn
	data IAmDispatcher
}

func handleCamera(conn net.Conn, newCameras chan Camera) {
	buf := make([]byte, 6)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		log.Printf("failed to read camera metadata from conn: %v", err)
		return
	}

	newCameras <- Camera{
		conn: conn,
		data: IAmCamera{
			road:  binary.BigEndian.Uint16(buf[0:2]),
			mile:  binary.BigEndian.Uint16(buf[2:4]),
			limit: binary.BigEndian.Uint16(buf[4:6]),
		},
	}

	for {
		// Go do camera things
	}
}

func handleDispatcher(conn net.Conn, newDispatchers chan Dispatcher) {}

func stateMachine(newCamera chan Camera, newDispatcher chan Dispatcher) {
	cameras := make(map[net.Conn]IAmCamera)
	dispatchers := make(map[net.Conn]IAmDispatcher)

	for {
		select {
		case c := <-newCamera:
			cameras[c.conn] = c.data
		case d := <-newDispatcher:
			dispatchers[d.conn] = d.data
		}
	}
}

func main() {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", port, err)
	}
	defer listener.Close()

	newCameras := make(chan Camera)
	newDispatchers := make(chan Dispatcher)
	go stateMachine(newCameras, newDispatchers)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
		}

		go func(c net.Conn) {
			defer c.Close()

			b, err := bufio.NewReader(c).ReadByte()
			if err != nil {
				log.Printf("failed to read msgType: %s", err)
				return
			}

			switch b {
			case 0x80:
				handleCamera(c, newCameras)
			case 0x81:
				handleDispatcher(c, newDispatchers)
			default:
				log.Printf("recieved invalid clientType: 0x%x", b)
			}
		}(conn)
	}
}
