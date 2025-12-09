package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"slices"
	"time"
)

const port = ":8080"

type event struct {
	conn   net.Conn
	msg    message
	signal chan struct{}
}

func sendErrorAndDisconnect(conn net.Conn, msg string) {
	defer conn.Close()
	var e Error
	e.msg.body = fmt.Append(e.msg.body, msg)
	err := e.encode(conn)
	if err != nil {
		log.Printf("failed to send Error to client %v: %v", conn, err)
	} else {
		log.Print(msg)
	}
}

func handleHeartbeat(conn net.Conn, msg *WantHeartbeat, signal chan struct{}) {
	fmt.Printf("client %v: hearbeat '%d'\n", conn, msg.Interval)
	if msg.Interval == 0 {
		return
	}

	var h Heartbeat
	for {
		time.Sleep(time.Duration(msg.Interval) * 100 * time.Millisecond)
		if err := h.encode(conn); err != nil {
			log.Printf("failed to send heartbeat to client %v: %v", conn, err)
			break
		}
	}
	fmt.Printf("client %v: done\n", conn)
	signal <- struct{}{}
}

func serverManager(events chan *event) {
	var heartbeats = make(map[net.Conn]*WantHeartbeat)
	var roads = make(map[uint16]*Road)

	for e := range events {
		switch msg := e.msg.(type) {
		case *Plate:
			if camera, ok := cameras[e.conn]; ok {
				road, ok := roads[camera.road]
				if !ok {
					log.Printf("road for camera %v does not exist", e.conn)
					continue
				}

				s := sighting{
					road:      camera.road,
					mile:      camera.mile,
					limit:     camera.limit,
					plate:     msg.plate,
					timestamp: msg.timestamp,
				}

				sightings := road.cars[string(s.plate.body)]
				sightings = append(sightings, &s)

				slices.SortFunc(sightings, func(a, b *sighting) int {
					if a.timestamp < b.timestamp {
						return -1
					}
					if a.timestamp > b.timestamp {
						return 1
					}
					return 0
				})
				idx := slices.Index(sightings, &s)

				tix := tickets[string(s.plate.body)]
				if _, ok := tix[s.timestamp/86400]; ok { // TODO need to check this logic
					continue // we have a ticket for this day already and should continue
				}

				curr := sightings[idx]

				if idx > 0 {
					left := sightings[idx-1]
					distance := curr.mile - left.mile
					time := curr.timestamp - left.timestamp
					speed := (float64(distance) / float64(time)) * 3600.0

					if speed > float64(curr.limit) {
						t := &Ticket{
							mile1:      left.mile,
							timestamp1: left.timestamp,
							mile2:      curr.mile,
							timestamp2: curr.timestamp,
							speed:      uint16(speed) * 100,
						}
						tix[t.timestamp1/86400] = struct{}{}
						tix[t.timestamp2/86400] = struct{}{}
						continue
					}
				}

				if idx < len(sightings) {
					right := sightings[idx+1]
					distance := curr.mile - right.mile
					time := curr.timestamp - right.timestamp
					speed := (float64(distance) / float64(time)) * 3600.0

					if speed > float64(curr.limit) {
						t := &Ticket{
							mile1:      right.mile,
							timestamp1: right.timestamp,
							mile2:      curr.mile,
							timestamp2: curr.timestamp,
							speed:      uint16(speed) * 100,
						}
						tix[t.timestamp1/86400] = struct{}{}
						tix[t.timestamp2/86400] = struct{}{}
					}
				}

			} else {
				sendErrorAndDisconnect(e.conn, fmt.Sprintf("Client: %v\nError: It is an error for a client that has not identified itself as a camera to send a Plate message.", e.conn))
			}
		case *WantHeartbeat:
			if _, ok := heartbeats[e.conn]; ok {
				sendErrorAndDisconnect(e.conn, fmt.Sprintf("Client: %v\nError: It is an error for a client to send multiple WantHeartbeat messages on a single connection", e.conn))
			} else {
				heartbeats[e.conn] = msg
				go handleHeartbeat(e.conn, msg, e.signal)
			}
		case *IAmCamera:
			if _, ok := cameras[e.conn]; ok {
				sendErrorAndDisconnect(e.conn, fmt.Sprintf("Client: %v\nError: It is an error for a client that has already identified itself as a camera to send an IAmCamera message.", e.conn))
				delete(cameras, e.conn)
			} else if _, ok := dispatchers[e.conn]; ok {
				sendErrorAndDisconnect(e.conn, fmt.Sprintf("Client: %v\nError: It is an error for a client that has already identified itself as a ticket dispatcher to send an IAmCamera message.", e.conn))
				delete(dispatchers, e.conn)
			} else {
				cameras[e.conn] = msg
			}
		case *IAmDispatcher:
			if _, ok := cameras[e.conn]; ok {
				sendErrorAndDisconnect(e.conn, fmt.Sprintf("Client: %v\nError: It is an error for a client that has already identified itself as a camera to send an IAmDispatcher message.", e.conn))
				delete(cameras, e.conn)
			} else if _, ok := dispatchers[e.conn]; ok {
				sendErrorAndDisconnect(e.conn, fmt.Sprintf("Client: %v\nError: It is an error for a client that has already identified itself as a ticket dispatcher to send an IAmDispatcher message.", e.conn))
				delete(dispatchers, e.conn)
			} else {
				dispatchers[e.conn] = msg
			}
		}
	}
}

func handleConnection(conn net.Conn, events chan *event) {
	defer conn.Close()

	var b uint8
	if err := binary.Read(conn, binary.BigEndian, &b); err != nil {
		log.Printf("failed to read msgType: %s", err)
		return
	}

	switch b {
	case byte(MsgWantHeartBeat):
		fmt.Printf("new heartbeat client: %v\n", conn)
		var m WantHeartbeat
		if err := m.decode(conn); err != nil {
			log.Printf("heartbeat setup failed for client %v: %v", conn, err)
			return
		}

		e := event{
			conn:   conn,
			msg:    &m,
			signal: make(chan struct{}),
		}
		events <- &e
		<-e.signal
	case byte(MsgIAmCamera):
		var m IAmCamera
		if err := m.decode(conn); err != nil {
			log.Printf("camera setup failed for client %v: %v", conn, err)
			return
		}

		e := event{
			conn:   conn,
			msg:    &m,
			signal: make(chan struct{}),
		}
		events <- &e
		<-e.signal
	case byte(MsgIAmDispatcher):
		var m IAmDispatcher
		if err := m.decode(conn); err != nil {
			log.Printf("dipatcher setup failed for client %v: %v", conn, err)
			return
		}

		e := event{
			conn:   conn,
			msg:    &m,
			signal: make(chan struct{}),
		}
		events <- &e
		<-e.signal
	default:
		sendErrorAndDisconnect(conn, fmt.Sprintf("Error: It is an error for a client to send the server a message with any message type value that is not listed below with 'Client->Server': 0x%x.", b))
	}
}

func main() {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", port, err)
	}
	defer listener.Close()

	events := make(chan *event)

	go serverManager(events)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
		}

		go handleConnection(conn, events)
	}
}
