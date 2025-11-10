package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"reflect"
)

var cameras map[net.Conn]*IAmCamera
var roads map[uint16][]*Plate

func newCameraFromConn(conn net.Conn, cameraEvents chan<- *event) error {
	var m IAmCamera
	err := m.decode(conn)
	if err != nil {
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
			err := p.decode(conn)
			if err != nil {
				log.Printf("0x%x: failed to decode: %v", typ, err)
				return
			}
			cameraEvents <- &event{conn: conn, msg: &p}
		case uint8(MsgWantHeartBeat):
			var w WantHeartbeat
			err := w.decode(conn)
			if err != nil {
				log.Printf("0x%x: failed to decode: %v", typ, err)
				return
			}
			cameraEvents <- &event{conn: conn, msg: &w}
		default:
			var e Error
			e.msg.body = fmt.Appendf(e.msg.body, "0x%x: invalid message type for client type camera", typ)
			log.Print(string(e.msg.body))
			err := e.encode(conn)
			if err != nil {
				log.Printf("failed to encode msg '%s' to client %v: %v", e.msg.body, conn, err)
			}
			return
		}
	}
}

func cameraManager(cameraEvents <-chan *event) {
	cameras = make(map[net.Conn]*IAmCamera)
	roads = make(map[uint16][]*Plate)

	// not sure if we need to close this channel anywhere? since it's technically supposed to run until the end of the program, it might be fine, but keep in mind this blocks forever
	for e := range cameraEvents {
		switch t := e.msg.(type) {
		case *IAmCamera:
			cameras[e.conn] = t
		case *Plate:
			camera, ok := cameras[e.conn]
			if !ok {
				log.Printf("camera for client %v does not exist", e.conn)
				return
			}
			plates := append(roads[camera.road], t)

			go handleTicketCheck(plates)
		case *WantHeartbeat:
			// need to keep track of these requests? a client can only request a heartbeat once. any further requests should trigger an Error
		case *ClientDisconnect:
			delete(cameras, e.conn)
		}
	}
}

func handleTicketCheck(plates []*Plate) {
	for _, p := range plates {
		if reflect.DeepEqual(p.plate, t.plate) {
		}
	}

}
