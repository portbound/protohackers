package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"unicode"
)

type room struct {
	clients        map[net.Conn]string
	connections    chan *client
	disconnections chan net.Conn
	broadcasts     chan string
}

type client struct {
	conn     net.Conn
	username string
}

func (r *room) connect(conn net.Conn) error {
	if _, err := conn.Write([]byte("Welcome to budgetchat! What shall I call you?")); err != nil {
		return err
	}

	username, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}

	// OPTIONAL
	if len(username) > 16 {
		return errors.New("username must be less than 16 chars")
	}

	for _, char := range username {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			return errors.New("username must be alphanumeric")
		}
	}

	// OPTIONAL
	if _, ok := r.clients[conn]; ok {
		return errors.New("username must be unique")
	}

	var users []string
	for _, username := range r.clients {
		users = append(users, username)
	}

	r.broadcast(fmt.Sprintf("* %s has entered the room", username))

	if _, err = conn.Write([]byte("* The room contains: " + strings.Join(users, ","))); err != nil {
		return err
	}

	r.connections <- &client{
		conn:     conn,
		username: username,
	}

	return nil
}

func (r *room) disconnect(conn net.Conn) {
	r.disconnections <- conn
}

func (r *room) broadcast(message string) {
}

func handleRoom(room *room) {
	select {
	case client := <-room.connections:
		room.clients[client.conn] = client.username
	case client := <-room.disconnections:
		delete(room.clients, client)
	case <-room.broadcasts:
	}
}

func handleConnection(conn net.Conn, room *room) {
	defer room.disconnect(conn)
	defer conn.Close()

	if err := room.connect(conn); err != nil {
		return
	}

	for {
		// select to send/receive from broadcasts
		// need some way to not wait forever though, or at least accept some kind of signal that client DC'd
	}
}

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to listen on port :8080: %v", err)
	}
	defer listener.Close()
	fmt.Println("listening on port :8080")

	room := room{
		clients: make(map[net.Conn]string),
	}

	go handleRoom(&room)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("failed to connect to listener on port :8080: %v", err)
		}

		go handleConnection(conn, &room)
	}
}
