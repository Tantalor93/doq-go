package doq

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// NewResolver creates a net.Resolver that sends DNS queries over QUIC.
func NewResolver(addr string, opts ...Option) *net.Resolver {
	return NewClient(addr, opts...).Resolver()
}

// Resolver creates a net.Resolver that sends DNS queries over QUIC.
func (c *Client) Resolver() *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return &resolverConn{
				client: c,
				ctx:    ctx,
			}, nil
		},
	}
}

type resolverConn struct {
	client *Client
	ctx    context.Context

	mu            sync.Mutex
	deadline      time.Time
	readDeadline  time.Time
	writeDeadline time.Time
	response      *bytes.Reader
	closed        bool
}

func (c *resolverConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, net.ErrClosed
	}
	if c.response == nil {
		return 0, io.EOF
	}

	return c.response.Read(p)
}

func (c *resolverConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, net.ErrClosed
	}
	if c.response != nil {
		c.mu.Unlock()
		return 0, errors.New("doq resolver connection can only handle a single request")
	}
	ctx, cancel := c.requestContextLocked()
	c.mu.Unlock()
	defer cancel()

	if len(p) < 2 {
		return 0, io.ErrShortBuffer
	}

	msgLen := int(binary.BigEndian.Uint16(p[:2]))
	if len(p[2:]) != msgLen {
		return 0, io.ErrShortBuffer
	}

	msg := &dns.Msg{}
	if err := msg.Unpack(p[2:]); err != nil {
		return 0, err
	}

	resp, err := c.client.Send(ctx, msg)
	if err != nil {
		return 0, err
	}

	pack, err := resp.Pack()
	if err != nil {
		return 0, err
	}

	respWithPrefix := make([]byte, 2+len(pack))
	// nolint:gosec
	binary.BigEndian.PutUint16(respWithPrefix, uint16(len(pack)))
	copy(respWithPrefix[2:], pack)

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, net.ErrClosed
	}
	c.response = bytes.NewReader(respWithPrefix)

	return len(p), nil
}

func (c *resolverConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.response = nil
	return nil
}

func (c *resolverConn) LocalAddr() net.Addr {
	return resolverAddr("doq-local")
}

func (c *resolverConn) RemoteAddr() net.Addr {
	return resolverAddr("doq-remote")
}

func (c *resolverConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deadline = t
	return nil
}

func (c *resolverConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readDeadline = t
	return nil
}

func (c *resolverConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writeDeadline = t
	return nil
}

func (c *resolverConn) requestContextLocked() (context.Context, context.CancelFunc) {
	deadline := earliestNonZero(c.deadline, c.readDeadline, c.writeDeadline)
	if deadline.IsZero() {
		return c.ctx, func() {}
	}

	if current, ok := c.ctx.Deadline(); ok && current.Before(deadline) {
		return c.ctx, func() {}
	}

	return context.WithDeadline(c.ctx, deadline)
}

func earliestNonZero(times ...time.Time) time.Time {
	var earliest time.Time
	for _, t := range times {
		if t.IsZero() {
			continue
		}
		if earliest.IsZero() || t.Before(earliest) {
			earliest = t
		}
	}
	return earliest
}

type resolverAddr string

func (a resolverAddr) Network() string {
	return "doq"
}

func (a resolverAddr) String() string {
	return string(a)
}
