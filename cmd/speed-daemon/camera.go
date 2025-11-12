package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"
)

var cameras map[net.Conn]*IAmCamera

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
	roads := make(map[uint16][]*Plate)
	heartbeats := make(map[net.Conn]uint32)

	// not sure if we need to close this channel anywhere? since it's technically supposed to run until the end of the program, it might be fine, but keep in mind this blocks forever
	for e := range cameraEvents {
		switch t := e.msg.(type) {
		case *IAmCamera:
			cameras[e.conn] = t
		case *Plate:
			camera, ok := cameras[e.conn]
			if !ok {
				log.Printf("camera for client %v does not exist", e.conn)
				continue
			}

			road, ok := roads[camera.road]
			if !ok {
				roads[camera.road] = append(roads[camera.road], t)
				continue
			}

			performTicketCheck(road, t)
		case *WantHeartbeat:
			if _, ok := heartbeats[e.conn]; ok {
				sendError(e.conn, fmt.Sprintf("Client: %v\nError: it is an error for a client to send multiple WantHeartbeat messages on a single connection", e.conn))
				cameraEvents <- &event{e.conn, &ClientDisconnect{}}
				continue
			}

			heartbeats[e.conn] = t.interval
			go handleHeartbeat(e.conn, t.interval)
		case *ClientDisconnect:
			delete(cameras, e.conn)
		}
	}
}

func handleHeartbeat(conn net.Conn, interval uint32) {
	for {
		time.Sleep(time.Duration(interval/10) * time.Second)
		var h Heartbeat
		if err := h.encode(conn); err != nil {
			log.Printf("failed to send heartbeat to client %v: %e", conn, err)
			return
		}
	}
}

func performTicketCheck([]*Plate, *Plate) {
	// if we have a current plate check for say 'mile 9' we're only interested in matching plates for 'mile 8' (I think?)
	// there's probably an algorithm we can use to streamline this, but i'd imagine the o(n) solution is probably fine for this challenge
	// We need to touch every plate to see if it's road number is our plate - 1 and then see if the plate actually matches
	// We do need to touch every plate though because the instructions say that clients may not send their plates on time, so the plates arent' actually sorted or anything
	// So yeah, I guess simple sort is probably pretty straightforward here
}
