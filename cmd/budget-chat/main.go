package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"slices"
	"unicode"
)

type chatRoom struct {
	users []string
}

func (cr *chatRoom) Join() bool {
	fmt.Println("Welcome to budgetchat! What shall I call you?")
	user, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false
	}

	// OPTIONAL
	if len(user) > 16 {
		return false
	}

	for _, char := range user {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			return false
		}
	}

	// OPTIONAL
	if slices.Contains(cr.users, user) {
		return false
	}

	// TODO broadcast here (?)

	cr.users = append(cr.users, user)
	return true
}

func handleConnection(conn net.Conn, room *chatRoom) {
	defer conn.Close()

	if !room.Join() {
		return
	}
}

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to listen on port :8080: %v", err)
	}
	defer listener.Close()
	fmt.Println("listening on port :8080")

	room := chatRoom{
		users: []string{},
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("failed to connect to listener on port :8080: %v", err)
		}

		go handleConnection(conn, &room)
	}
}
