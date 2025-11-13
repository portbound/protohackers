package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sort"
	"time"
)

var cameras map[net.Conn]*IAmCamera

type cust struct {
	*IAmCamera
	*Plate
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

	roads := make(map[uint16][]*cust)
	heartbeats := make(map[net.Conn]uint32)

	// not sure if we need to close this channel anywhere? since it's technically supposed to run until the end of the program, it might be fine, but keep in mind this blocks forever
	for e := range cameraEvents {
		switch msg := e.msg.(type) {
		case *IAmCamera:
			cameras[e.conn] = msg
		case *Plate:
			camera, ok := cameras[e.conn]
			if !ok {
				log.Printf("camera for client %v does not exist", e.conn)
				continue
			}

			// not sure if we need this check, it makes sense, we don't need to perform a ticket check if this is the first instance of a plate on that particular road, but I don't know if it really matters, and this is hard to understand at a glance. It's probably better to just "check" since it should be super fast with a single plate anyway

			// road, ok := roads[camera.road]
			// if !ok {
			// 	// roads[camera.road] = append(roads[camera.road], msg)
			// 	roads[camera.road] = append(roads[camera.road], &cust{
			// 		IAmCamera: camera,
			// 		Plate:     msg,
			// 	})
			// 	continue
			// }

			c := cust{
				IAmCamera: camera,
				Plate:     msg,
			}

			roads[camera.road] = append(roads[camera.road], &c)

			performTicketCheck(roads[camera.road], &c)
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

func performTicketCheck(road []*cust, c *cust) {
	sort.Slice(road, func(i int, j int) bool {
		return road[i].mile < road[j].mile
	})

	// Must be sorted!!!

	// cameras may glitch and fail to take the picture at a given mile mark
	// plate pics may come out of order
	// can't send tickets twice

	// with these constraints the solution seems to be as follows:
	// By sorting the set, we can basically just find the index of the passed in cust and then search left until we find the first occurence of that plate at a lower mile mark, and then search right until we find the first occurence at a higher mile mark.
	// Only write tickets that contain the passed in cust. Since this is new, there will never be a dupe.
	// the only place this falls apart I think is if we receive images for a car speeding at mile 4 and then 6. We write a ticket and move on. Then later on a picture for mile 5 comes in and we do our check which should now technically return 2 more tickets.
	// So in total, the car gets tickets for 4>5, 5>6, and also 4>6
	// this does not seem right

	// var speed int
	// for i := len(road) - 1; i >= 0; i-- {
	// 	if bytes.Equal(road[i].plate.body, plate.plate.body) {
	// }
	// }
}
