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
	// Road      uint16
	Mile uint16
	// Limit     uint16
	// Plate     str
	Timestamp uint32
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
	signal <- struct{}{}
}

func ticketCheck(s *Sighting, plt string, sightings []*Sighting, tickets map[string]map[uint32]struct{}, camera *IAmCamera) *Ticket {
	slices.SortFunc(sightings, func(a, b *Sighting) int {
		if a.Timestamp < b.Timestamp {
			return -1
		}
		if a.Timestamp > b.Timestamp {
			return 1
		}
		return 0
	})
	idx := slices.Index(sightings, s)

	tix, ok := tickets[plt]
	if !ok {
		tix = make(map[uint32]struct{})
		tickets[plt] = tix
	}
	if _, ok := tix[s.Timestamp/86400]; ok { // TODO need to check this logic
		return nil // we have a ticket for this day already and should continue
	}

	curr := sightings[idx]

	var t *Ticket
	if idx > 0 {
		fmt.Println("checking left")
		left := sightings[idx-1]
		distance := curr.Mile - left.Mile
		time := curr.Timestamp - left.Timestamp
		speed := (float64(distance) / float64(time)) * 3600.0
		if speed > float64(camera.Limit) {
			t = &Ticket{
				Mile1:      left.Mile,
				Timestamp1: left.Timestamp,
				Mile2:      curr.Mile,
				Timestamp2: curr.Timestamp,
				Speed:      uint16(speed) * 100,
			}
			tix[t.Timestamp1/86400] = struct{}{}
			tix[t.Timestamp2/86400] = struct{}{}
			return t
		}
	}

	if idx < len(sightings) {
		fmt.Println("checking right")
		right := sightings[idx+1]
		distance := right.Mile - curr.Mile
		time := right.Timestamp - curr.Timestamp
		speed := (float64(distance) / float64(time)) * 3600.0 // TODO need to check speed calc
		if speed > float64(camera.Limit) {
			t = &Ticket{
				Mile1:      curr.Mile,
				Timestamp1: curr.Timestamp,
				Mile2:      right.Mile,
				Timestamp2: right.Timestamp,
				Speed:      uint16(speed) * 100,
			}
			tix[t.Timestamp1/86400] = struct{}{}
			tix[t.Timestamp2/86400] = struct{}{}
			return t
		}
	}

	return nil
}
func serverManager(events chan *Event) {
	cameras := make(map[net.Conn]*IAmCamera)
	dispatchers := make(map[net.Conn]*IAmDispatcher)
	var tickets = make(map[string]map[uint32]struct{}) // map[plate]map[day]struct{}
	var heartbeats = make(map[net.Conn]*WantHeartbeat)
	var roads = make(map[uint16]map[string][]*Sighting)

	for e := range events {
		switch msg := e.Msg.(type) {
		case *Plate:
			if camera, ok := cameras[e.Conn]; ok {
				road, ok := roads[camera.Road]
				if !ok {
					roads[camera.Road] = make(map[string][]*Sighting)
					road = roads[camera.Road]
				}

				s := Sighting{
					Mile:      camera.Mile,
					Timestamp: msg.Timestamp,
				}
				// plate := str{
				// 	Len:  msg.Plate.Len,
				// 	Body: msg.Plate.Body,
				// }
				p := string(msg.Plate.Body)
				road[p] = append(road[p], &s)

				if len(road[p]) == 1 {
					continue
				}

				ticket := ticketCheck(&s, p, road[p], tickets, camera)
				ticket.Plate = str{
					Len:  msg.Plate.Len,
					Body: msg.Plate.Body,
				}
				ticket.Road = camera.Road
				fmt.Printf("%+v\n", ticket)
				for conn, dispatcher := range dispatchers {
					if slices.Contains(dispatcher.Roads, camera.Road) {
						if err := ticket.encode(conn); err != nil {
							log.Fatalf("failed to send ticket to client %v: %v", conn, err)
						}
						break
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
		case byte(MsgWantHeartBeat):
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
		case byte(MsgIAmCamera):
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
		case byte(MsgIAmDispatcher):
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
