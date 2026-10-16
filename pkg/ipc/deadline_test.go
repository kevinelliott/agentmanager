package ipc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

type failingDeadlineConn struct {
	net.Conn
	err   error
	wrote bool
}

func (c *failingDeadlineConn) SetDeadline(time.Time) error { return c.err }

func (c *failingDeadlineConn) Write([]byte) (int, error) {
	c.wrote = true
	return 0, errors.New("unexpected write")
}

func TestClientSendDoesNotWriteWhenDeadlineCannotBeSet(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	want := errors.New("deadline unavailable")
	conn := &failingDeadlineConn{Conn: left, err: want}
	client := &unixClient{connected: true, conn: newConnection(conn)}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := client.Send(ctx, nil); !errors.Is(err, want) {
		t.Fatalf("Send error = %v, want %v", err, want)
	}
	if conn.wrote {
		t.Fatal("request sent without the required deadline")
	}
}
