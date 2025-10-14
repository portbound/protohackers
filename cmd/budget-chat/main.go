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
	clients       map[net.Conn]string
	broadcastChan chan *msg
	clientChan    chan *client
	mu            sync.Mutex
}

type client struct {
	conn     net.Conn
	username string
	isActive bool
}

type msg struct {
	body string
	conn net.Conn
}

func (r *room) connect(conn net.Conn) (string, error) {
	if _, err := conn.Write([]byte("Welcome to budgetchat! What shall I call you?\n> ")); err != nil {
		return "", err
	}

	input, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return "", err
	}
	username := strings.TrimRight(input, "\n")

	// OPTIONAL
	if len(username) > 16 {
		return "", errors.New("username must be less than 16 chars")
	}

	for _, char := range username {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			return "", errors.New("username must be alphanumeric")
		}
	}

	// OPTIONAL
	// TODO remove locks
	// r.mu.Lock()
	// if _, ok := r.clients[conn]; ok {
	// 	return "", errors.New("username must be unique")
	// }
	// r.mu.Unlock()

	r.clientChan <- &client{
		conn:     conn,
		username: username,
		isActive: true,
	}

	r.broadcastChan <- &msg{
		body: fmt.Sprintf("\n* %s has entered the room\n", username),
		conn: nil,
	}

	// var users []string
	// // TODO remove locks
	// r.mu.Lock()
	// for _, u := range r.clients {
	// 	if u != username {
	// 		users = append(users, username)
	// 	}
	// }
	// r.mu.Unlock()

	var message string
	if len(r.clients) > 0 {
		message = fmt.Sprintf("\n* The room contains: %s\n", strings.Join(users, ", "))
	} else {
		message = "\n* The room is empty\n"
	}

	if _, err := conn.Write([]byte(message)); err != nil {
		return "", err
	}

	return username, nil
}

func (r *room) disconnect(client *client) {
	r.clientChan <- client
	r.broadcastChan <- &msg{
		body: fmt.Sprintf("* %s has left the room\n", client.username),
		conn: nil,
	}
}

func handleRoom(room *room) {
	for {
		select {
		case client := <-room.clientChan:
			if client.isActive {
				room.clients[client.conn] = client.username
			} else {
				delete(room.clients, client.conn)
			}
		case msg := <-room.broadcastChan:
			for c := range room.clients {
				if c != msg.conn {
					go func(cn net.Conn) {
						if _, err := cn.Write([]byte(msg.body)); err != nil {
							fmt.Printf("failed to write to connection: %v", err)
						}
					}(c)
				}
			}
		}
	}
}

func handleConnection(conn net.Conn, room *room) {
	defer conn.Close()

	username, err := room.connect(conn)
	if err != nil {
		return
	}
	defer room.disconnect(&client{
		conn:     conn,
		username: username,
		isActive: false,
	})

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		room.broadcastChan <- &msg{
			body: fmt.Sprintf("[%s] %s\n", username, scanner.Text()),
			conn: conn,
		}
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
		broadcastChan: make(chan *msg),
		clients:       make(map[net.Conn]string),
		clientChan:    make(chan *client),
		mu:            sync.Mutex{},
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
