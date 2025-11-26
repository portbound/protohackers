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
var tickets map[string]map[uint32]struct{} // map[plate]map[day]struct{}

type road struct {
	speedLimit   uint16
	allSightings map[string][]*sighting
}

type sighting struct {
	road      uint16
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
				road:      camera.road,
				mile:      camera.mile,
				timestamp: msg.timestamp,
			}
			sightings := road.allSightings[string(msg.plate.body)]
			sightings = append(sightings, &s)

			// sort the slice
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

			// check to see if we have a ticket for the day
			ts := tickets[string(msg.plate.body)]
			if _, ok := ts[s.timestamp/86400]; ok { // TODO need to check this logic
				continue // we have a ticket for this day already and should continue
			}

			t := performTicketCheck(idx, sightings, camera.limit)
			if t != nil {
				t.plate = msg.plate
				t.road = camera.road
				// dispatch ticket
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

func performTicketCheck(idx int, sightings []*sighting, limit uint16) *Ticket {
	curr := sightings[idx]

	if idx > 0 {
		left := sightings[idx-1]
		distance := curr.mile - left.mile
		time := curr.timestamp - left.timestamp
		speed := (float64(distance) / float64(time)) * 3600.0

		if speed > float64(limit) {
			return &Ticket{
				mile1:      left.mile,
				timestamp1: left.timestamp,
				mile2:      curr.mile,
				timestamp2: curr.timestamp,
				speed:      uint16(speed),
			}
		}
	}

	if idx < len(sightings) {
		right := sightings[idx+1]
		distance := curr.mile - right.mile
		time := curr.timestamp - right.timestamp
		speed := (float64(distance) / float64(time)) * 3600.0

		if speed > float64(limit) {
			return &Ticket{
				mile1:      right.mile,
				timestamp1: right.timestamp,
				mile2:      curr.mile,
				timestamp2: curr.timestamp,
				speed:      uint16(speed),
			}
		}
	}

	return nil
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
