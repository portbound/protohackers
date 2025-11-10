package main

import (
	"bufio"
	"log"
	"net"
)

const port = ":8080"

type event struct {
	conn net.Conn
	msg  message
}

func main() {
	listener, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", port, err)
	}
	defer listener.Close()

	cameraEvents := make(chan *event)
	go cameraManager(cameraEvents)

	dispatcherEvents := make(chan *event)
	go dispatcherManager(dispatcherEvents)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v", err)
		}

		go func(c net.Conn) {
			defer c.Close()

			b, err := bufio.NewReader(c).ReadByte()
			if err != nil {
				log.Printf("failed to read msgType: %s", err)
				return
			}

			switch b {
			case byte(MsgIAmCamera):
				if err := newCameraFromConn(conn, cameraEvents); err != nil {
					log.Printf("camera setup failed for client %v: %v", conn, err)
					return
				}
				handleCamera(c, cameraEvents)
			case byte(MsgIAmDispatcher):
				if err := newDispatcherFromConn(conn, dispatcherEvents); err != nil {
					log.Printf("dipatcher setup failed for client %v: %v", conn, err)
					return
				}
				handleDispatcher(c, dispatcherEvents)
			default:
				log.Printf("failed to set up client, msg type 0x%x is invalid", b)
			}
		}(conn)
	}
}
