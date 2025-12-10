package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type typ uint8

const (
	MsgError         typ = 0x10
	MsgPlate         typ = 0x20
	MsgTicket        typ = 0x21
	MsgWantHeartBeat typ = 0x40
	MsgHeartBeat     typ = 0x41
	MsgIAmCamera     typ = 0x80
	MsgIAmDispatcher typ = 0x81
)

// Message Protocol Spec
// each message declares its type in a single uint8

type message interface {
	encode(net.Conn) error
	decode(net.Conn) error
}

type str struct {
	Len  uint8
	Body []byte
}

func (s *str) encode(conn net.Conn) error { return nil }
func (s *str) decode(conn net.Conn) error {
	err := binary.Read(conn, binary.BigEndian, &s.Len)
	if err != nil {
		return fmt.Errorf("read str len err: %w", err)
	}

	s.Body = make([]byte, s.Len)
	_, err = io.ReadFull(conn, s.Body)
	if err != nil {
		return fmt.Errorf("read str body err: %w", err)
	}

	return nil
}

type Error struct {
	Msg str
}

func (e *Error) encode(conn net.Conn) error {
	err := e.Msg.encode(conn)
	if err != nil {
		return fmt.Errorf("failed to encode str: %w", err)
	}
	err = binary.Write(conn, binary.BigEndian, &e)
	if err != nil {
		return fmt.Errorf("failed to write Error: %w", err)
	}
	return nil
}
func (e *Error) decode(conn net.Conn) error { return nil }

type Plate struct {
	Plate     str
	Timestamp uint32
}

func (p *Plate) encode(conn net.Conn) error { return nil }
func (p *Plate) decode(conn net.Conn) error {
	err := p.Plate.decode(conn)
	if err != nil {
		return fmt.Errorf("failed to decode str: %w", err)
	}

	err = binary.Read(conn, binary.BigEndian, &p.Timestamp)
	if err != nil {
		return fmt.Errorf("failed to read timestamp: %w", err)
	}

	return nil
}

type Ticket struct {
	Plate      str
	Road       uint16
	Mile1      uint16
	Timestamp1 uint32
	Mile2      uint16
	Timestamp2 uint32
	Speed      uint16 // 100x mph
}

func (t *Ticket) encode(conn net.Conn) error {
	if err := binary.Write(conn, binary.BigEndian, t); err != nil {
		return err
	}
	return nil
}
func (t *Ticket) decode(conn net.Conn) error { return nil }

type WantHeartbeat struct {
	Interval uint32
}

func (w *WantHeartbeat) encode(conn net.Conn) error { return nil }
func (w *WantHeartbeat) decode(conn net.Conn) error {
	if err := binary.Read(conn, binary.BigEndian, w); err != nil {
		return err
	}

	return nil
}

type Heartbeat struct{}

func (h *Heartbeat) encode(conn net.Conn) error {
	if err := binary.Write(conn, binary.BigEndian, byte(MsgHeartBeat)); err != nil {
		return err
	}
	return nil
}
func (h *Heartbeat) decode(conn net.Conn) error { return nil }

type IAmCamera struct {
	Road  uint16
	Mile  uint16
	Limit uint16
}

func (c *IAmCamera) encode(conn net.Conn) error { return nil }
func (c *IAmCamera) decode(conn net.Conn) error {
	if err := binary.Read(conn, binary.BigEndian, c); err != nil {
		return err
	}
	return nil
}

type IAmDispatcher struct {
	Numroads uint8
	Roads    []uint16
}

func (d *IAmDispatcher) encode(conn net.Conn) error { return nil }
func (d *IAmDispatcher) decode(conn net.Conn) error {
	if err := binary.Read(conn, binary.BigEndian, &d.Numroads); err != nil {
		return err
	}

	d.Roads = make([]uint16, d.Numroads)

	if err := binary.Read(conn, binary.BigEndian, d.Roads); err != nil {
		return err
	}

	return nil
}

type ClientDisconnect struct{}

func (c *ClientDisconnect) encode(conn net.Conn) error { return nil }
func (c *ClientDisconnect) decode(conn net.Conn) error { return nil }
