package doq_test

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
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
			go d.serveConn(conn)
		}
	}()
}

func (d *doqServer) stop() {
	if !d.closed.Swap(true) {
		_ = d.listener.Close()
	}
}

func (d *doqServer) serveConn(conn *quic.Conn) {
	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			if !errors.Is(err, io.EOF) && !d.closed.Load() {
				panic(err)
			}
			return
		}

		go d.serveStream(stream)
	}
}

func (d *doqServer) serveStream(stream *quic.Stream) {
	req, err := readQuery(stream)
	if err != nil {
		panic(err)
	}

	// to reliably test read timeout
	time.Sleep(time.Second)

	resp := dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 req.Id,
			Response:           true,
			Authoritative:      true,
			RecursionAvailable: true,
			Rcode:              dns.RcodeSuccess,
		},
		Question: req.Question,
	}

	for _, question := range req.Question {
		switch question.Qtype {
		case dns.TypeA:
			resp.Answer = append(resp.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    10,
				},
				A: net.ParseIP("127.0.0.1").To4(),
			})
		case dns.TypeAAAA:
			resp.Answer = append(resp.Answer, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeAAAA,
					Class:  dns.ClassINET,
					Ttl:    10,
				},
				AAAA: net.ParseIP("::1"),
			})
		}
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

func readQuery(stream *quic.Stream) (*dns.Msg, error) {
	sizeBuf := make([]byte, 2)
	if _, err := io.ReadFull(stream, sizeBuf); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint16(sizeBuf)
	buf := make([]byte, size)
	if _, err := io.ReadFull(stream, buf); err != nil {
		return nil, err
	}

	msg := &dns.Msg{}
	if err := msg.Unpack(buf); err != nil {
		return nil, err
	}

	return msg, nil
}
