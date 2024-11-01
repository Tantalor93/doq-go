package doq_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tantalor93/doq-go/doq"
)

type doqServer struct {
	addr     string
	listener *quic.Listener
	closed   atomic.Bool
}

func (d *doqServer) start() {
	listener, err := quic.ListenAddr("localhost:0", generateTLSConfig(), nil)
	if err != nil {
		panic(err)
	}
	d.listener = listener
	d.addr = listener.Addr().String()
	go func() {
		for {
			conn, err := listener.Accept(context.Background())
			if err != nil {
				if !d.closed.Load() {
					panic(err)
				}
				return
			}

			stream, err := conn.AcceptStream(context.Background())
			if err != nil {
				panic(err)
			}

			// to reliably test read timeout
			time.Sleep(time.Second)

			resp := dns.Msg{
				MsgHdr:   dns.MsgHdr{Rcode: dns.RcodeSuccess},
				Question: []dns.Question{{Name: "example.org.", Qtype: dns.TypeA}},
				Answer: []dns.RR{&dns.A{
					Hdr: dns.RR_Header{
						Name:   "example.org.",
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    10,
					},
					A: net.ParseIP("127.0.0.1"),
				}},
			}
			pack, err := resp.Pack()
			if err != nil {
				panic(err)
			}
			packWithPrefix := make([]byte, 2+len(pack))
			binary.BigEndian.PutUint16(packWithPrefix, uint16(len(pack)))
			copy(packWithPrefix[2:], pack)
			_, _ = stream.Write(packWithPrefix)
			_ = stream.Close()
		}
	}()
}

func (d *doqServer) stop() {
	if !d.closed.Swap(true) {
		_ = d.listener.Close()
	}
}

func Test(t *testing.T) {
	server := doqServer{}
	server.start()
	defer server.stop()

	client := doq.NewClient(server.addr, doq.WithTLSConfig(generateTLSConfig()))

	msg := dns.Msg{}
	msg.SetQuestion("example.org.", dns.TypeA)
	resp, err := client.Send(context.Background(), &msg)

	require.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, resp.Rcode)
	assert.Len(t, resp.Answer, 1)
	assert.Equal(t, net.ParseIP("127.0.0.1").To4(), resp.Answer[0].(*dns.A).A)
}

func TestWriteTimeout(t *testing.T) {
	server := doqServer{}
	server.start()
	defer server.stop()

	client := doq.NewClient(server.addr, doq.WithTLSConfig(generateTLSConfig()), doq.WithWriteTimeout(time.Nanosecond))

	msg := dns.Msg{}
	msg.SetQuestion("example.org.", dns.TypeA)
	resp, err := client.Send(context.Background(), &msg)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, resp)
}

func TestReadTimeout(t *testing.T) {
	server := doqServer{}
	server.start()
	defer server.stop()

	client := doq.NewClient(server.addr, doq.WithTLSConfig(generateTLSConfig()), doq.WithReadTimeout(time.Nanosecond))

	msg := dns.Msg{}
	msg.SetQuestion("example.org.", dns.TypeA)
	resp, err := client.Send(context.Background(), &msg)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, resp)
}

func generateTLSConfig() *tls.Config {
	cert, err := tls.LoadX509KeyPair("test.crt", "test.key")
	if err != nil {
		panic(err)
	}

	certs, err := os.ReadFile("test.crt")
	if err != nil {
		panic(err)
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		panic(err)
	}
	pool.AppendCertsFromPEM(certs)

	return &tls.Config{
		ServerName:   "localhost",
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"doq"},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS12,
	}
}
