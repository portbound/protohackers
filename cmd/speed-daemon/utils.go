package main

import (
	"fmt"
	"log"
	"net"
)

func sendError(conn net.Conn, msg string) {
	var e Error
	e.msg.body = fmt.Appendf(e.msg.body, msg)
	err := e.encode(conn)
	if err != nil {
		log.Printf("failed to send Error to client %v: %v", conn, err)
	} else {
		log.Print(msg)
	}
}
