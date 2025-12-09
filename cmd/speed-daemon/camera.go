package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"slices"
)

var cameras map[net.Conn]*IAmCamera
var tickets map[string]map[uint32]struct{} // map[plate]map[day]struct{}

type Road struct {
	cars map[string][]*sighting // map[plate]sightings
}

type sighting struct {
	road      uint16
	mile      uint16
	limit     uint16
	plate     str
	timestamp uint32
}

func newCameraFromConn(conn net.Conn, events chan<- *event) error {
	var m IAmCamera
	if err := m.decode(conn); err != nil {
		return err
	}

	events <- &event{conn: conn, msg: &m}
	return nil
} // func newCameraFromConn(conn net.Conn, cameraEvents chan<- *event) error {
// 	var m IAmCamera
// 	if err := m.decode(conn); err != nil {
// 		return err
// 	}
//
// 	cameraEvents <- &event{conn: conn, msg: &m}
// 	return nil
// }

func handleCamera(conn net.Conn, cameraEvents chan<- *event) {
	defer func() {
		cameraEvents <- &event{
			conn:   conn,
			msg:    nil,
			signal: make(chan struct{}),
		}
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
			sendErrorAndDisconnect(conn, fmt.Sprintf("Client: %v\nError: it is an error for a client to send the server a message with message type: 0x%x", conn, typ))
			return
		}
	}
}

func cameraManager(cameraEvents, dispatcherEvents chan *event) {
	var roads = make(map[uint16]*Road)
	for e := range cameraEvents {
		switch msg := e.msg.(type) {
		case *IAmCamera:
			cameras[e.conn] = msg
			if _, ok := roads[msg.road]; !ok {
				roads[msg.road] = &Road{
					cars: make(map[string][]*sighting),
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

			s := sighting{
				road:      camera.road,
				mile:      camera.mile,
				limit:     camera.limit,
				plate:     msg.plate,
				timestamp: msg.timestamp,
			}

			// don't need to check if it exists since a nil slice can be appended to (in the event that the plate doesn't exist yet)
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
			// idx := slices.Index(sightings, &s)

			tix := tickets[string(s.plate.body)]
			if _, ok := tix[s.timestamp/86400]; ok { // TODO need to check this logic
				continue // we have a ticket for this day already and should continue
			}

			// t := ticketCheck(idx, sightings)
			// tix[t.timestamp1/86400] = struct{}{}
			// tix[t.timestamp2/86400] = struct{}{}
			// dispatcherEvents <- &event{e.conn, t}

		case *ClientDisconnect:
			delete(cameras, e.conn)
		}
	}
}
