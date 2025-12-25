package doq_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tantalor93/doq-go/doq"
)

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
