package doq

import (
	"context"
	"crypto/tls"
	"io"
	"sync"

	"github.com/miekg/dns"
	"github.com/quic-go/quic-go"
)

// Client encapsulates and provides logic for querying DNS servers over QUIC.
type Client struct {
	sync.Mutex
	addr      string
	tlsconfig *tls.Config
	conn      quic.Connection
}

type Options struct {
	InsecureSkipVerify bool
}

func NewClient(addr string, options Options) (*Client, error) {
	client := Client{}

	client.addr = addr

	client.tlsconfig = &tls.Config{
		InsecureSkipVerify: options.InsecureSkipVerify,
		NextProtos:         []string{"doq"},
	}

	if err := client.dial(); err != nil {
		return nil, err
	}

	return &client, nil
}

func (c *Client) dial() error {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	if c.conn != nil {
		if err := c.conn.Context().Err(); err == nil {
			// somebody else created the connection in the meantime, no need to do anything
			return nil
		}
	}
	conn, err := quic.DialAddrEarly(c.addr, c.tlsconfig, nil)
	if err != nil {
		return err
	}

	c.conn = conn

	return nil
}

// Send sends DNS request using DNS over QUIC
func (c *Client) Send(ctx context.Context, msg *dns.Msg) (*dns.Msg, error) {
	pack, err := msg.Pack()
	if err != nil {
		return nil, err
	}

	// connection is not healthy, try to dial a new one
	if err := c.conn.Context().Err(); err != nil {
		if err := c.dial(); err != nil {
			return nil, err
		}
	}

	streamSync, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}

	_, err = streamSync.Write(pack)
	// close the stream to indicate we are done sending or the server might wait till we close the stream or timeout is hit
	_ = streamSync.Close()
	if err != nil {
		return nil, err
	}

	buf, err := io.ReadAll(streamSync)
	if err != nil {
		return nil, err
	}

	resp := dns.Msg{}
	if err := resp.Unpack(buf); err != nil {
		return nil, err
	}
	return &resp, nil
}
