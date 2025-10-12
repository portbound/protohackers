package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
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
	if _, err := conn.Write([]byte("Welcome to budgetchat! What shall I call you?\n")); err != nil {
		return err
	}

	input, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return err
	}
	username := strings.TrimRight(input, "\n")

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

	r.broadcast(fmt.Sprintf("* %s has entered the room", username), nil)

	var users []string
	for _, username := range r.clients {
		users = append(users, username)
	}

	var message string
	if len(users) > 0 {
		message = "* The room contains: " + strings.Join(users, ",")
	} else {
		message = "* The room is empty"
	}

	if _, err = conn.Write([]byte(message)); err != nil {
		return err
	}

	r.connections <- &client{
		conn:     conn,
		username: username,
	}

	return nil
}

func (r *room) disconnect(conn net.Conn) {
	// r.disconnections <- conn
	// idk maybe this works? We might just wanna do a select
	mu := sync.Mutex{}
	mu.Lock()
	delete(r.clients, conn)
	mu.Unlock()
}

// revisit this - it works, but feels too nested
func (r *room) broadcast(message string, conn net.Conn) {
	for c := range r.clients {
		if c != conn {
			go func(cn net.Conn) {
				if _, err := cn.Write([]byte(message)); err != nil {
					fmt.Printf("failed to write to connection: %v", err)
				}
			}(c)
		}
	}
}

func handleRoom(room *room) {
	// not sure if we need to handle some sort of kill signal for the loop
	for {
		select {
		case client := <-room.connections:
			room.clients[client.conn] = client.username
		case client := <-room.disconnections:
			delete(room.clients, client)
		case <-room.broadcasts:
		}
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
		clients:        make(map[net.Conn]string),
		connections:    make(chan *client),
		disconnections: make(chan net.Conn),
		broadcasts:     make(chan string),
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
