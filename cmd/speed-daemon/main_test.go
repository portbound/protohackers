package main

import (
	"net"
	"reflect"
	"testing"
	"time"
)

func TestHandleConnection(t *testing.T) {
	tests := []struct {
		name          string
		input         []byte
		expectMsgType Message
		validate      func(t *testing.T, msg Message)
	}{
		{
			name:  "IAmCamera message",
			input: []byte{0x80, 0x00, 0x7b, 0x00, 0x08, 0x00, 0x3c},
			validate: func(t *testing.T, msg Message) {
				camera, ok := msg.(*IAmCamera)
				if !ok {
					t.Fatalf("expected *IAmCamera, got: %T", msg)
				}
				if camera.Road != 123 {
					t.Errorf("want: Road 123, got: %d", camera.Road)
				}
				if camera.Mile != 8 {
					t.Errorf("want: Mile 8, got: %d", camera.Mile)
				}
				if camera.Limit != 60 {
					t.Errorf("want: Limit 60, got: %d", camera.Limit)
				}
			},
		},
		{
			name:  "Plate message",
			input: []byte{0x20, 0x04, 0x55, 0x4e, 0x31, 0x58, 0x00, 0x00, 0x03, 0xe8},
			validate: func(t *testing.T, msg Message) {
				plate, ok := msg.(*Plate)
				if !ok {
					t.Fatalf("expected *Plate, got: %T", msg)
				}

				wPlate := "UN1X"
				gPlate := string(plate.Plate.Body)
				if wPlate != gPlate {
					t.Errorf("want: %s, got: %s", wPlate, gPlate)
				}

				wTimestamp := uint32(45)
				gTimestamp := plate.Timestamp
				if wTimestamp != gTimestamp {
					t.Errorf("want: %d, got: %d", wTimestamp, gTimestamp)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, client := net.Pipe()
			defer server.Close()
			defer client.Close()

			events := make(chan *Event, 1)
			go HandleConnection(server, events)

			_, err := client.Write(tt.input)
			if err != nil {
				t.Fatalf("failed to write to client connection: %v", err)
			}

			select {
			case event := <-events:

			case <-time.After(1 * time.Second):
				t.Fatalf("timed out waiting for event from HandleConnection")
			}
		})
	}
}

func TestServer_HandleEvent(t *testing.T) {
	tests := []struct {
		name     string
		msg      Message
		validate func(t *testing.T, e *Event, s *Server)
	}{
		{
			name: "Camera Msg Received: Valid",
			msg:  &IAmCamera{Road: 123, Mile: 8, Limit: 60},
			validate: func(t *testing.T, e *Event, s *Server) {
				if len(s.Cameras) != 1 {
					t.Fatalf("expected 1 camera, got %d", len(s.Cameras))
				}
				camera, ok := s.Cameras[e.Conn]
				if !ok {
					t.Fatalf("failed to add client to list of cameras")
				}

				if !reflect.DeepEqual(camera, e.Msg) {
					t.Fatalf("want: %+v, got: %+v", e.Msg, camera)
				}
			},
		},
		{
			name: "Dispatcher Msg Recieved: Valid",
		},
		{
			name: "WantHeartBeat Msg Received: Valid",
		},
		{
			name: "Plate Msg Received: Valid",
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
			tt.validate(t, e, s)
		})
	}
}
