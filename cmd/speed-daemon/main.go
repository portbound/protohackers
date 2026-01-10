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
	Mile      uint16
	Timestamp uint32
}

type Event struct {
	Conn net.Conn
	Msg  Message
}

type Server struct {
	Cameras     map[net.Conn]*IAmCamera
	Dispatchers map[net.Conn]*IAmDispatcher
	Heartbeats  map[net.Conn]*WantHeartbeat
	Roads       map[uint16]map[string][]*Sighting
	Tickets     map[string]map[uint32]struct{}
}

func NewServer() *Server {
	return &Server{
		Cameras:     make(map[net.Conn]*IAmCamera),
		Dispatchers: make(map[net.Conn]*IAmDispatcher),
		Heartbeats:  make(map[net.Conn]*WantHeartbeat),
		Roads:       make(map[uint16]map[string][]*Sighting), // map[road num]map[plate]*[]Sighting
		Tickets:     make(map[string]map[uint32]struct{}),    // map[plate]map[day]struct{}
	}
}

func (s *Server) Run(events chan *Event) {
	for e := range events {
		s.HandleEvent(e)
	}
}

func (s *Server) HandleEvent(e *Event) {
	switch msg := e.Msg.(type) {
	case *Plate:
		camera := s.Cameras[e.Conn]

		road, ok := s.Roads[camera.Road]
		if !ok {
			s.Roads[camera.Road] = make(map[string][]*Sighting)
			road = s.Roads[camera.Road]
		}

		sighting := Sighting{
			Mile:      camera.Mile,
			Timestamp: msg.Timestamp,
		}

		plate := msg.Plate
		road[string(plate.Body)] = append(road[string(plate.Body)], &sighting)

		if len(road[string(plate.Body)]) == 1 {
			break
		}

		// make a chan and pass this ticket into it for the ticketManager
		// this needs to be created in main though so events and ticketManager/Processor can communicate
		ch := make(chan *Ticket)
		// if the ticket is nil, we can ignore this in the ticketManager
		ch <- ticketCheck(&sighting, plate, road[string(plate.Body)], s.Tickets, camera)

		// ticket := ticketCheck(&sighting, plate, road[string(plate.Body)], s.Tickets, camera)
		// offload this logic to the ticket checker
		// ticket.Plate = Str{
		// 	Len:  msg.Plate.Len,
		// 	Body: msg.Plate.Body,
		// }
		// ticket.Road = camera.Road
		// for conn, dispatcher := range s.Dispatchers {
		// 	if slices.Contains(dispatcher.Roads, camera.Road) {
		// 		if err := ticket.encode(conn); err != nil {
		// 			log.Fatalf("failed to send ticket to client %v: %v", conn, err)
		// 		}
		// 		break
		// 	}
		// }

	case *WantHeartbeat:
		s.Heartbeats[e.Conn] = msg
		go handleHeartbeat(e.Conn, msg)
	case *IAmCamera:
		s.Cameras[e.Conn] = msg
	case *IAmDispatcher:
		s.Dispatchers[e.Conn] = msg
	case *ClientDisconnect:
		defer e.Conn.Close()

		if msg.clientIdentity == byte(MsgIAmCamera) {
			delete(s.Cameras, e.Conn)
		}

		if msg.clientIdentity == byte(MsgIAmDispatcher) {
			delete(s.Dispatchers, e.Conn)
		}

		var m ErrorMessage
		m.Str.Body = fmt.Append(m.Str.Body, msg)
		err := m.encode(e.Conn)
		if err != nil {
			log.Printf("failed to send ErrorMessage to client %v: %v", e.Conn, err)
		} else {
			log.Println(msg)
		}
	}
}

func handleHeartbeat(conn net.Conn, msg *WantHeartbeat) {
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
}

func ticketCheck(s *Sighting, plt Str, sightings []*Sighting, tickets map[string]map[uint32]struct{}, camera *IAmCamera) *Ticket {
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

	tix, ok := tickets[string(plt.Body)]
	if !ok {
		tix = make(map[uint32]struct{})
		tickets[string(plt.Body)] = tix
	}
	if _, ok := tix[s.Timestamp/86400]; ok { // TODO need to check this logic
		return nil // we have a ticket for this day already and should continue
	}

	curr := sightings[idx]

	var t *Ticket
	if idx > 0 {
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
		right := sightings[idx+1]
		distance := right.Mile - curr.Mile
		time := right.Timestamp - curr.Timestamp
		speed := (float64(distance) / float64(time)) * 3600.0 // TODO need to check speed calc
		if speed > float64(camera.Limit) {
			t = &Ticket{
				Mile1:      curr.Mile,
				Timestamp1: curr.Timestamp,
				Mile2:      right.Mile,
				Timestamp2: curr.Timestamp,
				Speed:      uint16(speed) * 100,
			}
			tix[t.Timestamp1/86400] = struct{}{}
			tix[t.Timestamp2/86400] = struct{}{}
			return t
		}
	}

	return nil
}

func HandleConnection(conn net.Conn, events chan *Event) {
	defer conn.Close()

	var clientIdentity uint8
	var requestedHeartbeat bool

	for {
		var b uint8

		if err := binary.Read(conn, binary.BigEndian, &b); err != nil {
			log.Printf("failed to read msgType: %s", err)
			return
		}

		switch b {
		case byte(MsgPlate):
			if clientIdentity != byte(MsgIAmCamera) {
				err := "It is an error for a client that has not identified itself as a camera  to send a Plate message."
				events <- &Event{
					Conn: conn,
					Msg: &ClientDisconnect{
						clientIdentity: clientIdentity,
						errMsg:         err,
					},
				}
				log.Printf("disconnected client %v: %s", conn, err)
				return
			}
			var m Plate
			if err := m.decode(conn); err != nil {
				log.Printf("failed to decode plate for client %v: %v", conn, err)
				return
			}
		case byte(MsgWantHeartBeat):
			if requestedHeartbeat {
				err := "It is an error for a client to send multiple WantHeartbeat messages on a single connection"
				events <- &Event{
					Conn: conn,
					Msg: &ClientDisconnect{
						clientIdentity: clientIdentity,
						errMsg:         err,
					},
				}
				log.Printf("disconnected client %v: %s", conn, err)
				return
			}
			requestedHeartbeat = true

			var m WantHeartbeat
			if err := m.decode(conn); err != nil {
				log.Printf("heartbeat setup failed for client %v: %v", conn, err)
				return
			}

			events <- &Event{
				Conn: conn,
				Msg:  &m,
			}
		case byte(MsgIAmCamera):
			if clientIdentity != 0 {
				err := "It is an error for a client that has already identified itself as either a camera or a ticket dispatcher to send an IAmCamera message."
				events <- &Event{
					Conn: conn,
					Msg: &ClientDisconnect{
						clientIdentity: clientIdentity,
						errMsg:         err,
					},
				}
				log.Printf("disconnected client %v: %s", conn, err)
				return
			}
			clientIdentity = byte(MsgIAmCamera)

			var m IAmCamera
			if err := m.decode(conn); err != nil {
				log.Printf("camera setup failed for client %v: %v\n", conn, err)
				return
			}

			events <- &Event{
				Conn: conn,
				Msg:  &m,
			}
		case byte(MsgIAmDispatcher):
			if clientIdentity != 0 {
				err := "It is an error for a client that has already identified itself as either a camera or a ticket dispatcher to send an IAmCamera message."
				events <- &Event{
					Conn: conn,
					Msg:  &ClientDisconnect{clientIdentity: clientIdentity, errMsg: err},
				}
				log.Printf("disconnected client %v: %s", conn, err)
				return
			}
			clientIdentity = byte(MsgIAmDispatcher)

			var m IAmDispatcher
			if err := m.decode(conn); err != nil {
				log.Printf("dipatcher setup failed for client %v: %v\n", conn, err)
				return
			}

			events <- &Event{
				Conn: conn,
				Msg:  &m,
			}
		default:
			err := fmt.Sprintf("received an invalid message type: %d from client %v", b, conn)
			events <- &Event{
				Conn: conn,
				Msg: &ClientDisconnect{
					clientIdentity: clientIdentity,
					errMsg:         err,
				},
			}
			log.Printf("disconnected client %v: %s", conn, err)
			return
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

	server := NewServer()
	go server.Run(events)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
		}

		go HandleConnection(conn, events)
	}
}
