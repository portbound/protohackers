package main

import (
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"
)

// func TestHandleConnection(t *testing.T) {
// 	tests := []struct {
// 		name          string
// 		input         []byte
// 		expectMsgType Message
// 		validate      func(t *testing.T, msg Message)
// 	}{
// 		{
// 			name:  "IAmCamera message",
// 			input: []byte{0x80, 0x00, 0x7b, 0x00, 0x08, 0x00, 0x3c},
// 			validate: func(t *testing.T, msg Message) {
// 				camera, ok := msg.(*IAmCamera)
// 				if !ok {
// 					t.Fatalf("expected *IAmCamera, got: %T", msg)
// 				}
// 				if camera.Road != 123 {
// 					t.Errorf("want: Road 123, got: %d", camera.Road)
// 				}
// 				if camera.Mile != 8 {
// 					t.Errorf("want: Mile 8, got: %d", camera.Mile)
// 				}
// 				if camera.Limit != 60 {
// 					t.Errorf("want: Limit 60, got: %d", camera.Limit)
// 				}
// 			},
// 		},
// 		{
// 			name:  "Plate message",
// 			input: []byte{0x20, 0x04, 0x55, 0x4e, 0x31, 0x58, 0x00, 0x00, 0x03, 0xe8},
// 			validate: func(t *testing.T, msg Message) {
// 				plate, ok := msg.(*Plate)
// 				if !ok {
// 					t.Fatalf("expected *Plate, got: %T", msg)
// 				}
//
// 				wPlate := "UN1X"
// 				gPlate := string(plate.Plate.Body)
// 				if wPlate != gPlate {
// 					t.Errorf("want: %s, got: %s", wPlate, gPlate)
// 				}
//
// 				wTimestamp := uint32(45)
// 				gTimestamp := plate.Timestamp
// 				if wTimestamp != gTimestamp {
// 					t.Errorf("want: %d, got: %d", wTimestamp, gTimestamp)
// 				}
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			server, client := net.Pipe()
// 			defer server.Close()
// 			defer client.Close()
//
// 			events := make(chan *Event, 1)
// 			go HandleConnection(server, events)
//
// 			_, err := client.Write(tt.input)
// 			if err != nil {
// 				t.Fatalf("failed to write to client connection: %v", err)
// 			}
//
// 			select {
// 			case event := <-events:
//
// 			case <-time.After(1 * time.Second):
// 				t.Fatalf("timed out waiting for event from HandleConnection")
// 			}
// 		})
// 	}
// }

func TestServer_HandleEvent(t *testing.T) {
	tests := []struct {
		name     string
		msg      Message
		validate func(t *testing.T, e *Event, s *Server, c net.Conn)
	}{
		{
			name: "Camera Msg Received: Happy Path",
			msg:  &IAmCamera{Road: 123, Mile: 8, Limit: 60},
			validate: func(t *testing.T, e *Event, s *Server, c net.Conn) {
				if len(s.Cameras) != 1 {
					t.Fatalf("expected 1 camera, got %d", len(s.Cameras))
				}
				camera, ok := s.Cameras[e.Conn]
				if !ok {
					t.Fatalf("failed to add client as camera")
				}

				if !reflect.DeepEqual(e.Msg, camera) {
					t.Fatalf("want: %+v, got: %+v", e.Msg, camera)
				}
			},
		},
		{
			name: "Dispatcher Msg Recieved: Happy Path",
			msg: &IAmDispatcher{
				Numroads: 1,
				Roads:    []uint16{123},
			},
			validate: func(t *testing.T, e *Event, s *Server, c net.Conn) {
				if len(s.Dispatchers) != 1 {
					t.Fatalf("expected 1 dispatcher, got %d", len(s.Dispatchers))
				}
				dispatcher, ok := s.Dispatchers[e.Conn]
				if !ok {
					t.Fatalf("failed to add client as dispatcher")
				}
				if !reflect.DeepEqual(e.Msg, dispatcher) {
					t.Fatalf("want: %+v, got: %+v", e.Msg, dispatcher)
				}
			},
		},
		{
			name: "WantHeartBeat Msg Received: Happy Path",
			msg: &WantHeartbeat{
				Interval: 2,
			},
			validate: func(t *testing.T, e *Event, s *Server, c net.Conn) {
				type readResult struct {
					data []byte
					err  error
				}
				ch := make(chan readResult)
				go func() {
					for {
						hb := make([]byte, 1)
						_, err := c.Read(hb)
						ch <- readResult{data: hb, err: err}
						if err != nil {
							return
						}
					}
				}()
				for i := range 4 {
					select {
					case result := <-ch:
						if result.err != nil {
							t.Fatalf("read failed: %v", result.err)
						}
						fmt.Println(i)
					case <-time.After(2 * time.Second):
						t.Fatalf("timeout waiting for a heartbeat")
					}
				}
			},
		},
		{
			name: "Plate Msg Received: Happy Path",
			msg:  &Plate{},
		},
		{
			name: "Plate Msg Received: Send a Ticket",
			msg:  &Plate{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer()
			c1, c2 := net.Pipe()
			defer c1.Close()
			defer c2.Close()

			e := &Event{
				Conn:   c2,
				Msg:    tt.msg,
				Signal: make(chan struct{}),
			}

			s.HandleEvent(e)
			tt.validate(t, e, s, c1)
		})
	}
}
