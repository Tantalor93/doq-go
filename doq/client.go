package doq

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"io"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/quic-go/quic-go"
)

// Client encapsulates and provides logic for querying DNS servers over QUIC.
// The client should be thread-safe. The client reuses single QUIC connection to the server, while creating multiple parallel QUIC streams.
type Client struct {
	connLock sync.RWMutex
	conn     *quic.Conn

	addr           string
	tlsConfig      *tls.Config
	writeTimeout   time.Duration
	readTimeout    time.Duration
	connectTimeout time.Duration
}

// NewClient creates a new doq.Client used for sending DoQ queries.
func NewClient(addr string, opts ...Option) *Client {
	client := &Client{
		addr:      addr,
		tlsConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
	for _, opt := range opts {
		opt.apply(client)
	}

	// override protocol negotiation to DoQ, all the other stuff (like certificates, cert pools, insecure skip)
	// is up to the user of library
	client.tlsConfig.NextProtos = []string{"doq"}

	return client
}

func (c *Client) dial(ctx context.Context) error {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	if c.conn != nil {
		c.conn.ConnectionState()
		if err := c.conn.Context().Err(); err == nil {
			// somebody else created the connection in the meantime, no need to do anything
			return nil
		}
	}

	connectCtx := ctx
	if c.connectTimeout != 0 {
		var cancel context.CancelFunc
		connectCtx, cancel = context.WithTimeout(connectCtx, c.connectTimeout)
		defer cancel()
	}

	done := make(chan interface{})

	go func() {
		conn, err := quic.DialAddrEarly(connectCtx, c.addr, c.tlsConfig, nil)
		if err != nil {
			done <- err
			return
		}
		done <- conn
	}()

	select {
	case <-connectCtx.Done():
		return connectCtx.Err()
	case res := <-done:
		switch r := res.(type) {
		case error:
			return r
		case *quic.Conn:
			c.conn = r
		}
	}

	return nil
}

// Send sends DNS request using DNS over QUIC.
func (c *Client) Send(ctx context.Context, msg *dns.Msg) (*dns.Msg, error) {
	if err := c.dialIfNeeded(ctx); err != nil {
		return nil, err
	}

	stream, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}

	writeCtx := ctx
	if c.writeTimeout != 0 {
		var cancel context.CancelFunc
		writeCtx, cancel = context.WithTimeout(writeCtx, c.writeTimeout)
		defer cancel()
	}

	readCtx := ctx
	if c.readTimeout != 0 {
		var cancel context.CancelFunc
		readCtx, cancel = context.WithTimeout(readCtx, c.readTimeout)
		defer cancel()
	}

	// Single watchdog: cancels both stream directions when either the write or
	// read context expires. Exits without cancelling when Send returns normally.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-writeCtx.Done():
		case <-readCtx.Done():
		case <-done:
			return
		}
		stream.CancelWrite(0)
		stream.CancelRead(0)
	}()

	if err := writeCtx.Err(); err != nil {
		return nil, err
	}
	if err := writeMsg(stream, msg); err != nil {
		if writeCtx.Err() != nil {
			return nil, writeCtx.Err()
		}
		return nil, err
	}

	if err := readCtx.Err(); err != nil {
		return nil, err
	}
	resp, err := readMsg(stream)
	if err != nil {
		if readCtx.Err() != nil {
			return nil, readCtx.Err()
		}
		return nil, err
	}
	return resp, nil
}

func (c *Client) dialIfNeeded(ctx context.Context) error {
	c.connLock.RLock()
	connNotCreated := c.conn == nil
	c.connLock.RUnlock()

	if connNotCreated {
		// connection not yet created, create one
		if err := c.dial(ctx); err != nil {
			return err
		}
	}

	c.connLock.RLock()
	connFailed := c.conn.Context().Err() != nil
	c.connLock.RUnlock()

	if connFailed {
		// connection is not healthy, try to dial a new one
		if err := c.dial(ctx); err != nil {
			return err
		}
	}
	return nil
}

func writeMsg(stream *quic.Stream, msg *dns.Msg) error {
	pack, err := msg.Pack()
	if err != nil {
		return err
	}
	packWithPrefix := make([]byte, 2+len(pack))
	// nolint:gosec
	binary.BigEndian.PutUint16(packWithPrefix, uint16(len(pack)))
	copy(packWithPrefix[2:], pack)

	_, err = stream.Write(packWithPrefix)
	if err != nil {
		return err
	}
	// close the stream to indicate we are done sending or the server might wait till we close the stream or timeout is hit
	_ = stream.Close()
	return nil
}

func readMsg(stream *quic.Stream) (*dns.Msg, error) {
	// read 2-octet length field to know how long the DNS message is
	sizeBuf := make([]byte, 2)
	_, err := io.ReadFull(stream, sizeBuf)
	if err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint16(sizeBuf)
	buf := make([]byte, size)
	_, err = io.ReadFull(stream, buf)
	if err != nil {
		return nil, err
	}

	resp := dns.Msg{}
	if err := resp.Unpack(buf); err != nil {
		return nil, err
	}
	return &resp, nil
}
