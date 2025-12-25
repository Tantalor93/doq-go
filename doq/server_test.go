package doq_test

import (
	"context"
	"encoding/binary"
	"net"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"github.com/quic-go/quic-go"
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
			// nolint:gosec
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
