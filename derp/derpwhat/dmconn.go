package derpwhat

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
	"tailscale.com/derp"
)

var _ derp.Conn = &dmconn{}

// dmconn represents a full-duplex dm flow, presenting it as a derp.Conn
type dmconn struct {
	jid    types.JID
	client *whatsmeow.Client

	rxQ   chan []byte
	rxBuf bytes.Buffer
}

func (c *dmconn) rxPump() {
	go func() {
		for b := range c.rxQ {
			c.rxBuf.Write(b)
		}
	}()
}

func (c *dmconn) Write(p []byte) (int, error) {
	// Send view once message to c.JID

	// XXX: is this correct?
	msg := &waProto.Message{ViewOnceMessage: &waProto.FutureProofMessage{
		Message: &waProto.Message{Conversation: proto.String(string(p))},
	}}

	_, err := c.client.SendMessage(
		context.TODO(),
		c.jid,
		"",
		msg,
	)

	return len(p), err
}

func (c *dmconn) Close() error {
	close(c.rxQ)

	// TODO: set-presence to some kind of gone
	// TODO: send goodbye?

	// TODO: de-register with derpwhat
	return nil
}

// brw creates a new bufio.ReadWriter for the connection
func (c *dmconn) brw() *bufio.ReadWriter {
	return bufio.NewReadWriter(bufio.NewReader(&c.rxBuf), bufio.NewWriter(c))
}

type WhatAddr struct {
	jid types.JID
}

func (wa *WhatAddr) Network() string {
	return "whatsapp"
}

func (wa *WhatAddr) String() string {
	return wa.jid.String()
}

func (c *dmconn) LocalAddr() net.Addr {
	return &WhatAddr{c.jid}
}

// The *Deadline methods follow the semantics of net.Conn.
func (c *dmconn) SetDeadline(time.Time) error {
	// panic("TODO")
	return nil
}

func (c *dmconn) SetReadDeadline(time.Time) error {
	// panic("TODO")
	return nil
}

func (c *dmconn) SetWriteDeadline(time.Time) error {
	// panic("TODO")
	return nil
}
