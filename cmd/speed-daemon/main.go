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

// roads are identified as 0 - 65535
// roads have a single speed limit
// road positions are identified by # of miles from start of road

// DATA TYPE
// type str is a length-prefixed byte slice
// 0-255 length ASCII char codes
type str struct {
	len  uint8
	body []byte
}

// MESSAGES
type message interface {
	isMessage()
}

// each message begins with a sing u8
type Error struct { // 0x10
	msg str
}

func (e *Error) isMessage()

type Plate struct { // 0x20
	plate     str
	timestamp uint32
}

func (p *Plate) isMessage()

type Ticket struct { // 0x21
	plate      str
	road       uint16
	mile1      uint16
	timestamp1 uint32
	mile2      uint16
	timestamp2 uint32
	speed      uint16 // 100x mph
}

func (t *Ticket) isMessage()

type WantHeartbeat struct { // 0x40
	interval uint32
}

func (wh *WantHeartbeat) isMessage()

type Heartbeat struct{} // 0x41
func (h *Heartbeat) isMessage()

// cameras report each plate they observe, along with a timestamp
// timestamps are unix timestamps that are unisgned
type IAmCamera struct { // 0x80
	road  uint16
	mile  uint16
	limit uint16
}

func (c *IAmCamera) isMessage()

// when server detects car speeding at 2 points on the same road, it finds the corresponding dispatcher and has it deliver a ticket
type IAmDispatcher struct { // 0x81
	numroads uint8
	roads    []uint16
}

func (d *IAmDispatcher) isMessage()

type ClientDisconnect struct {
	conn net.Conn
}

func (c *ClientDisconnect) isMessage()

type Camera struct {
	conn     net.Conn
	data     IAmCamera
	msgTypes map[uint8]struct{}
}

type Dispatcher struct {
	conn     net.Conn
	data     IAmDispatcher
	msgTypes map[uint8]struct{}
}

func newCameraFromConn(conn net.Conn) (*Camera, error) {
	buf := make([]byte, 6)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read camera metadata from conn: %v", err)
	}

	return &Camera{
		conn: conn,
		data: IAmCamera{
			road:  binary.BigEndian.Uint16(buf[0:2]),
			mile:  binary.BigEndian.Uint16(buf[2:4]),
			limit: binary.BigEndian.Uint16(buf[4:6]),
		},
	}, nil
}

func handleCamera(c *Camera, cameraEvents chan message) {
	defer func() {
		cameraEvents <- &ClientDisconnect{conn: c.conn}
	}()

	r := bufio.NewReader(c.conn)
	for {
		msgType, err := r.ReadByte()
		if err != nil {
			log.Printf("connection to camera was lost: %v", err)
			return
		}

		switch msgType {
		case 0x20:
			var p Plate
			strLen, err := r.ReadByte()
			if err != nil {
				log.Printf("0x20: failed to read str len: %v", err)
				return
			}
			p.plate.len = strLen

			body := make([]byte, p.plate.len)
			_, err = io.ReadFull(c.conn, body)
			if err != nil {
				log.Printf("0x20: failed to read str: %v", err)
				return
			}
			copy(p.plate.body, body)

			timestamp := make([]byte, 4)
			_, err = io.ReadFull(c.conn, timestamp)
			if err != nil {
				log.Printf("0x20: failed to read timestamp: %v", err)
				return
			}
			p.timestamp = binary.BigEndian.Uint32(timestamp)

			cameraEvents <- &p
		case 0x40:
			cameraEvents <- &WantHeartbeat{}
			continue
		}

		// b is not a valid message type, handle error
	}
}

func cameraManager(newCamera <-chan *Camera, cameraEvents <-chan message) {
	cameras := make(map[net.Conn]IAmCamera)

	for {
		select {
		case c := <-newCamera:
			cameras[c.conn] = c.data
		case event := <-cameraEvents:
			switch e := event.(type) {
			case *Plate:
			// register plate and check to see if we have a potential ticket
			case *WantHeartbeat:
			// send heartbeat
			case *ClientDisconnect:
				delete(cameras, e.conn)
			}
		}
	}
}

func newDispatcherFromConn(c net.Conn) (*Dispatcher, error) {
	panic("unimplemented")
}

func handleDispatcher(dispatcher *Dispatcher, dispatcherEvents chan message) {
	panic("unimplemented")
}

func dispatcherManager(newDispatchers chan *Dispatcher, dispatcherEvents chan message) {
	panic("unimplemented")
}

func main() {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", port, err)
	}
	defer listener.Close()

	newCameras := make(chan *Camera)
	cameraEvents := make(chan message)
	go cameraManager(newCameras, cameraEvents)

	newDispatchers := make(chan *Dispatcher)
	dispatcherEvents := make(chan message)
	go dispatcherManager(newDispatchers, dispatcherEvents)

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
				camera, err := newCameraFromConn(c)
				if err != nil {
					log.Printf("failed to setup Camera for %v: %v", c, err)
					return
				}
				newCameras <- camera
				handleCamera(camera, cameraEvents)
			case 0x81:
				dispatcher, err := newDispatcherFromConn(c)
				if err != nil {
					log.Printf("failed to setup Dispatcher for %v: %v", c, err)
					return
				}
				newDispatchers <- dispatcher
				handleDispatcher(dispatcher, dispatcherEvents)
			default:
				log.Printf("failed to set up client, msg type 0x%x is invalid", b)
			}
		}(conn)
	}
}
