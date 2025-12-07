package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

const port = ":8080"

type event struct {
	conn   net.Conn
	msg    message
	signal chan struct{}
}

func sendError(conn net.Conn, msg string) {
	var e Error
	e.msg.body = fmt.Append(e.msg.body, msg)
	err := e.encode(conn)
	if err != nil {
		log.Printf("failed to send Error to client %v: %v", conn, err)
	} else {
		log.Print(msg)
	}
}

func handleHeartbeat(conn net.Conn, interval uint32) {
	var h Heartbeat
	for {
		time.Sleep(time.Duration(interval/10) * time.Second)
		if err := h.encode(conn); err != nil {
			log.Printf("failed to send heartbeat to client %v: %v", conn, err)
			return
		}
	}
}

func ticketCheck(idx int, sightings []*sighting) *Ticket {
	curr := sightings[idx]

	if idx > 0 {
		left := sightings[idx-1]
		distance := curr.mile - left.mile
		time := curr.timestamp - left.timestamp
		speed := (float64(distance) / float64(time)) * 3600.0

		if speed > float64(curr.limit) {
			return &Ticket{
				mile1:      left.mile,
				timestamp1: left.timestamp,
				mile2:      curr.mile,
				timestamp2: curr.timestamp,
				speed:      uint16(speed) * 100,
			}
		}
	}

	if idx < len(sightings) {
		right := sightings[idx+1]
		distance := curr.mile - right.mile
		time := curr.timestamp - right.timestamp
		speed := (float64(distance) / float64(time)) * 3600.0

		if speed > float64(curr.limit) {
			return &Ticket{
				mile1:      right.mile,
				timestamp1: right.timestamp,
				mile2:      curr.mile,
				timestamp2: curr.timestamp,
				speed:      uint16(speed) * 100,
			}
		}
	}
	return nil
}

func serverManager(events chan *event, sig chan struct{}) {
	var heartbeats = make(map[net.Conn]*WantHeartbeat)

	for e := range events {
		switch msg := e.msg.(type) {
		case *Plate:
		case *WantHeartbeat:
			if _, ok := heartbeats[e.conn]; ok {
				sendError(e.conn, fmt.Sprintf("Client: %v\nError: It is an error for a client to send multiple WantHeartbeat messages on a single connection", e.conn))
				e.conn.Close()
			} else {
				heartbeats[e.conn] = msg
				go handleHeartbeat(e.conn, msg.Interval)
			}
			e.signal <- struct{}{}
		case *IAmCamera:
			if _, ok := cameras[e.conn]; ok {
				sendError(e.conn, fmt.Sprintf("Client: %v\nError: It is an error for a client that has already identified itself as either a camera or a ticket dispatcher to send an IAmCamera message.", e.conn))
				e.conn.Close()
			} else {
				cameras[e.conn] = msg
			}
			e.signal <- struct{}{}
		case *IAmDispatcher:
			if _, ok := dispatchers[e.conn]; ok {
				sendError(e.conn, fmt.Sprintf("Client: %v\nError: It is an error for a client that has already identified itself as either a camera or a ticket dispatcher to send an IAmDispatcher message.", e.conn))
				e.conn.Close()
			} else {
				dispatchers[e.conn] = msg
			}
			e.signal <- struct{}{}
		}
	}
}

func main() {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", port, err)
	}
	defer listener.Close()

	events := make(chan *event)
	sig := make(chan struct{})

	go serverManager(events, sig)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
		}

		go handleConnection(conn, events)
	}
}

func handleConnection(conn net.Conn, events chan *event) {
	defer conn.Close()

	var b uint8
	if err := binary.Read(conn, binary.BigEndian, &b); err != nil {
		log.Printf("failed to read msgType: %s", err)
		return
	}

	for {
		switch b {
		case byte(MsgIAmCamera):
			var m IAmCamera
			if err := m.decode(conn); err != nil {
				log.Printf("camera setup failed for client %v: %v", conn, err)
			}

			e := event{conn: conn, msg: &m}
			events <- &e
			<-e.signal
		case byte(MsgIAmDispatcher):
			var m IAmDispatcher
			if err := m.decode(conn); err != nil {
				log.Printf("dipatcher setup failed for client %v: %v", conn, err)
			}

			e := event{conn: conn, msg: &m}
			events <- &e
			<-e.signal
		case byte(MsgWantHeartBeat):
			var m WantHeartbeat
			if err := m.decode(conn); err != nil {
				log.Printf("heartbeat setup failed for client %v: %v", conn, err)
				return
			}

			e := event{conn: conn, msg: &m}
			events <- &e
			<-e.signal
		default:
			log.Printf("failed to set up client, msg type 0x%x is invalid", b)
		}
	}
}
