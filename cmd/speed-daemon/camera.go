package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"slices"
	"time"
)

var cameras map[net.Conn]*IAmCamera

type road struct {
	speedLimit   uint16
	allSightings map[string][]*sighting
}

type sighting struct {
	mile      uint16
	timestamp uint32
}

func newCameraFromConn(conn net.Conn, cameraEvents chan<- *event) error {
	var m IAmCamera
	if err := m.decode(conn); err != nil {
		return err
	}

	cameraEvents <- &event{conn: conn, msg: &m}
	return nil
}

func handleCamera(conn net.Conn, cameraEvents chan<- *event) {
	defer func() {
		cameraEvents <- &event{conn, &ClientDisconnect{}}
	}()

	var typ uint8
	for {
		err := binary.Read(conn, binary.BigEndian, &typ)
		if err != nil {
			log.Printf("failed to read message type from client %v: %e", conn, err)
			return
		}

		switch typ {
		case uint8(MsgPlate):
			var p Plate
			if err := p.decode(conn); err != nil {
				log.Printf("0x%x: failed to decode: %v", typ, err)
				return
			}
			cameraEvents <- &event{conn: conn, msg: &p}
		case uint8(MsgWantHeartBeat):
			var w WantHeartbeat
			if err := w.decode(conn); err != nil {
				log.Printf("0x%x: failed to decode: %v", typ, err)
				return
			}
			cameraEvents <- &event{conn: conn, msg: &w}
		default:
			sendError(conn, fmt.Sprintf("Client: %v\nError: it is an error for a client to send the server a message with message type: 0x%x", conn, typ))
			return
		}
	}
}

func cameraManager(cameraEvents chan *event) {

	var heartbeats = make(map[net.Conn]uint32)
	var allTickets = make(map[string]map[uint32]struct{}) // map[plate]map[day]struct{}
	var roads = make(map[uint16]*road)

	// not sure if we need to close this channel anywhere? since it's technically supposed to run until the end of the program, it might be fine, but keep in mind this blocks forever
	for e := range cameraEvents {
		switch msg := e.msg.(type) {
		case *IAmCamera:
			cameras[e.conn] = msg
			if _, ok := roads[msg.road]; !ok {
				roads[msg.road] = &road{
					speedLimit:   msg.limit,
					allSightings: make(map[string][]*sighting),
				}
			}
		case *Plate:
			camera, ok := cameras[e.conn]
			if !ok {
				log.Printf("camera for client %v does not exist", e.conn)
				continue
			}

			road, ok := roads[camera.road]
			if !ok {
				log.Printf("road for camera %v does not exist", e.conn)
				continue
			}

			// create sighting and add to plate's slice
			s := sighting{
				mile:      camera.mile,
				timestamp: msg.timestamp,
			}
			sightings := road.allSightings[string(msg.plate.body)]
			sightings = append(sightings, &s)

			// // check if we have tickets for the current plate
			// tickets, ok := allTickets[string(msg.plate.body)]
			// if ok {
			// 	// if we do, check if any of them are for the current day
			// 	if _, ok = tickets[s.timestamp/86400]; ok {
			// 		// if one exists, move on
			// 		continue
			// 	}
			// }

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
			left := idx - 1
			right := idx + 1

			// after sorting, when we check the left, if we have a ticket we don't need to check the right. All potential combinations touching &s are now "marked" as having a ticket
			// we only need to check right when leftTicket comes back as nil, meaning we didn't find one on the left
			leftTicket := performTicketCheck(&s, sightings[left], camera.limit)
			if leftTicket != nil {
				// dispatch ticket
				continue
			}

			rightTicket := performTicketCheck(&s, sightings[right], camera.limit)
			if rightTicket != nil {
			}

		case *WantHeartbeat:
			if _, ok := heartbeats[e.conn]; ok {
				sendError(e.conn, fmt.Sprintf("Client: %v\nError: it is an error for a client to send multiple WantHeartbeat messages on a single connection", e.conn))
				cameraEvents <- &event{e.conn, &ClientDisconnect{}}
				continue
			}

			heartbeats[e.conn] = msg.interval
			go handleHeartbeat(e.conn, msg.interval)
		case *ClientDisconnect:
			delete(cameras, e.conn)
		}
	}
}

func performTicketCheck(curr, neighbor *sighting, limit uint16) *Ticket {
	i := curr
	j := neighbor

	if curr.mile < neighbor.mile {
		i = neighbor
		j = curr
	}

	distance := i.mile - j.mile
	time := i.timestamp - j.timestamp
	speed := (float64(distance) / float64(time)) * 3600.0

	var ticket Ticket
	if speed > float64(limit) {
		// if i.timestamp/86400 != j.timestamp/86400 then we're dealing with potentially two tickets
		// check if i.timestamp/86400 has an existing ticket in tickets slice if it doesn't, create the ticket
		// check if j.timestamp/86400 has an existing ticket in tickets slice if it doesn't, create the ticket

		// I worry that waiting to check here is adding unneccessary compute to the program. By this point, we've already sorted the slice and checked for speeding
	}

	return &ticket
}

func handleHeartbeat(conn net.Conn, interval uint32) {
	var h Heartbeat
	for {
		time.Sleep(time.Duration(interval/10) * time.Second)
		if err := h.encode(conn); err != nil {
			log.Printf("failed to send heartbeat to client %v: %e", conn, err)
			return
		}
	}
}
