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

type Road struct {
	Cars map[string][]*Sighting // map[plate]sightings
}

type Sighting struct {
	Road      uint16
	Mile      uint16
	Limit     uint16
	Plate     str
	timestamp uint32
}
type Event struct {
	Conn   net.Conn
	Msg    message
	Signal chan struct{}
}

func sendErrorAndDisconnect(conn net.Conn, msg string) {
	defer conn.Close()
	var e Error
	e.Msg.Body = fmt.Append(e.Msg.Body, msg)
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

func serverManager(events chan *Event) {
	cameras := make(map[net.Conn]*IAmCamera)
	dispatchers := make(map[net.Conn]*IAmDispatcher)
	var tickets map[string]map[uint32]struct{} // map[plate]map[day]struct{}
	var heartbeats = make(map[net.Conn]*WantHeartbeat)
	var roads = make(map[uint16]*Road)

	for e := range events {
		switch msg := e.Msg.(type) {
		case *Plate:
			if camera, ok := cameras[e.Conn]; ok {
				road, ok := roads[camera.Road]
				if !ok {
					log.Printf("road for camera %v does not exist", e.Conn)
					continue
				}

				s := Sighting{
					Road:      camera.Road,
					Mile:      camera.Mile,
					Limit:     camera.Limit,
					Plate:     msg.Plate,
					timestamp: msg.Timestamp,
				}

				sightings := road.Cars[string(s.Plate.Body)]
				sightings = append(sightings, &s)

				slices.SortFunc(sightings, func(a, b *Sighting) int {
					if a.timestamp < b.timestamp {
						return -1
					}
					if a.timestamp > b.timestamp {
						return 1
					}
					return 0
				})
				idx := slices.Index(sightings, &s)

				tix := tickets[string(s.Plate.Body)]
				if _, ok := tix[s.timestamp/86400]; ok { // TODO need to check this logic
					continue // we have a ticket for this day already and should continue
				}

				curr := sightings[idx]

				if idx > 0 {
					left := sightings[idx-1]
					distance := curr.Mile - left.Mile
					time := curr.timestamp - left.timestamp
					speed := (float64(distance) / float64(time)) * 3600.0

					if speed > float64(curr.Limit) {
						t := &Ticket{
							Mile1:      left.Mile,
							Timestamp1: left.timestamp,
							Mile2:      curr.Mile,
							Timestamp2: curr.timestamp,
							Speed:      uint16(speed) * 100,
						}
						tix[t.Timestamp1/86400] = struct{}{}
						tix[t.Timestamp2/86400] = struct{}{}
						continue
					}
				}

				if idx < len(sightings) {
					right := sightings[idx+1]
					distance := curr.Mile - right.Mile
					time := curr.timestamp - right.timestamp
					speed := (float64(distance) / float64(time)) * 3600.0

					if speed > float64(curr.Limit) {
						t := &Ticket{
							Mile1:      right.Mile,
							Timestamp1: right.timestamp,
							Mile2:      curr.Mile,
							Timestamp2: curr.timestamp,
							Speed:      uint16(speed) * 100,
						}
						tix[t.Timestamp1/86400] = struct{}{}
						tix[t.Timestamp2/86400] = struct{}{}
					}
				}

			} else {
				sendErrorAndDisconnect(e.Conn, fmt.Sprintf("Client: %v\nError: It is an error for a client that has not identified itself as a camera to send a Plate message.", e.Conn))
			}
		case *WantHeartbeat:
			if _, ok := heartbeats[e.Conn]; ok {
				sendErrorAndDisconnect(e.Conn, fmt.Sprintf("Client: %v\nError: It is an error for a client to send multiple WantHeartbeat messages on a single connection", e.Conn))
			} else {
				heartbeats[e.Conn] = msg
				go handleHeartbeat(e.Conn, msg, e.Signal)
			}
		case *IAmCamera:
			if _, ok := cameras[e.Conn]; ok {
				sendErrorAndDisconnect(e.Conn, fmt.Sprintf("Client: %v\nError: It is an error for a client that has already identified itself as a camera to send an IAmCamera message.", e.Conn))
				delete(cameras, e.Conn)
			} else if _, ok := dispatchers[e.Conn]; ok {
				sendErrorAndDisconnect(e.Conn, fmt.Sprintf("Client: %v\nError: It is an error for a client that has already identified itself as a ticket dispatcher to send an IAmCamera message.", e.Conn))
				delete(dispatchers, e.Conn)
			} else {
				cameras[e.Conn] = msg
				e.Signal <- struct{}{}
			}
		case *IAmDispatcher:
			if _, ok := cameras[e.Conn]; ok {
				sendErrorAndDisconnect(e.Conn, fmt.Sprintf("Client: %v\nError: It is an error for a client that has already identified itself as a camera to send an IAmDispatcher message.", e.Conn))
				delete(cameras, e.Conn)
			} else if _, ok := dispatchers[e.Conn]; ok {
				sendErrorAndDisconnect(e.Conn, fmt.Sprintf("Client: %v\nError: It is an error for a client that has already identified itself as a ticket dispatcher to send an IAmDispatcher message.", e.Conn))
				delete(dispatchers, e.Conn)
			} else {
				dispatchers[e.Conn] = msg
				e.Signal <- struct{}{}
			}
		}
	}
}

func handleConnection(conn net.Conn, events chan *Event) {
	defer conn.Close()

	var b uint8
	for {
		if err := binary.Read(conn, binary.BigEndian, &b); err != nil {
			log.Printf("failed to read msgType: %s", err)
			return
		}

		switch b {
		case byte(MsgPlate):
			fmt.Println("GOT A PLATE")
			var m Plate
			if err := m.decode(conn); err != nil {
				log.Printf("failed to decode plate for client %v: %v", conn, err)
				return
			}
			e := Event{
				Conn:   conn,
				Msg:    &m,
				Signal: make(chan struct{}),
			}
			events <- &e
			<-e.Signal
			fmt.Println("plate done???")
		case byte(MsgWantHeartBeat):
			fmt.Printf("new heartbeat client: %v\n", conn)
			var m WantHeartbeat
			if err := m.decode(conn); err != nil {
				log.Printf("heartbeat setup failed for client %v: %v", conn, err)
				return
			}

			e := Event{
				Conn:   conn,
				Msg:    &m,
				Signal: make(chan struct{}),
			}
			events <- &e
			<-e.Signal
			fmt.Println("new heartbeat client SETUP COMPLETE")
		case byte(MsgIAmCamera):
			fmt.Printf("new camera client: %v\n", conn)
			var m IAmCamera
			if err := m.decode(conn); err != nil {
				log.Printf("camera setup failed for client %v: %v\n", conn, err)
				return
			}

			e := Event{
				Conn:   conn,
				Msg:    &m,
				Signal: make(chan struct{}),
			}
			events <- &e
			<-e.Signal
			fmt.Println("new camera client SETUP COMPLETE")
		case byte(MsgIAmDispatcher):
			fmt.Printf("new dispatcher client: %v\n", conn)
			var m IAmDispatcher
			if err := m.decode(conn); err != nil {
				log.Printf("dipatcher setup failed for client %v: %v\n", conn, err)
				return
			}

			e := Event{
				Conn:   conn,
				Msg:    &m,
				Signal: make(chan struct{}),
			}
			events <- &e
			<-e.Signal
			fmt.Println("new dispatcher client SETUP COMPLETE")
		default:
			sendErrorAndDisconnect(conn, fmt.Sprintf("Error: It is an error for a client to send the server a message with any message type value that is not listed below with 'Client->Server': 0x%x.\n", b))
		}
	}
}

func main() {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", port, err)
	}
	defer listener.Close()

	events := make(chan *Event)

	go serverManager(events)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
		}

		go handleConnection(conn, events)
	}
}
