package doq

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"io"
	"sync"

	"github.com/miekg/dns"
	"github.com/quic-go/quic-go"
)

// Client encapsulates and provides logic for querying DNS servers over QUIC.
// The client should be thread-safe. The client reuses single QUIC connection to the server, while creating multiple parallel QUIC streams.
type Client struct {
	sync.Mutex
	addr      string
	tlsconfig *tls.Config
	conn      quic.Connection
}

// Options encapsulates configuration options for doq.Client.
type Options struct {
	TLSConfig *tls.Config
}

// NewClient creates a new doq.Client used for sending DoQ queries.
func NewClient(addr string, options Options) (*Client, error) {
	client := Client{}

	client.addr = addr

	if options.TLSConfig == nil {
		client.tlsconfig = &tls.Config{MinVersion: tls.VersionTLS12}
	} else {
		client.tlsconfig = options.TLSConfig.Clone()
	}

	// override protocol negotiation to DoQ, all the other stuff (like certificates, cert pools, insecure skip) is up to the user of library
	client.tlsconfig.NextProtos = []string{"doq"}

	return &client, nil
}

func (c *Client) dial(ctx context.Context) error {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	if c.conn != nil {
		if err := c.conn.Context().Err(); err == nil {
			// somebody else created the connection in the meantime, no need to do anything
			return nil
		}
	}
	conn, err := quic.DialAddrEarly(ctx, c.addr, c.tlsconfig, nil)
	if err != nil {
		return err
	}

	c.conn = conn

	return nil
}

// Send sends DNS request using DNS over QUIC.
func (c *Client) Send(ctx context.Context, msg *dns.Msg) (*dns.Msg, error) {
	if c.conn == nil {
		// connection not yet created, create one
		if err := c.dial(ctx); err != nil {
			return nil, err
		}
	}
	if err := c.conn.Context().Err(); err != nil {
		// connection is not healthy, try to dial a new one
		if err := c.dial(ctx); err != nil {
			return nil, err
		}
	}

	pack, err := msg.Pack()
	if err != nil {
		return nil, err
	}
	packWithPrefix := make([]byte, 2+len(pack))
	binary.BigEndian.PutUint16(packWithPrefix, uint16(len(pack)))
	copy(packWithPrefix[2:], pack)

	streamSync, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}

	_, err = streamSync.Write(packWithPrefix)
	// close the stream to indicate we are done sending or the server might wait till we close the stream or timeout is hit
	_ = streamSync.Close()
	if err != nil {
		return nil, err
	}

	// read 2-octet length field to know how long the DNS message is
	sizeBuf := make([]byte, 2)
	_, err = io.ReadFull(streamSync, sizeBuf)
	if err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint16(sizeBuf)
	buf := make([]byte, size)
	_, err = io.ReadFull(streamSync, buf)
	if err != nil {
		return nil, err
	}

	resp := dns.Msg{}
	if err := resp.Unpack(buf); err != nil {
		return nil, err
	}
	return &resp, nil
}
