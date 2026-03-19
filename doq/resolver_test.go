package doq_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tantalor93/doq-go/doq"
)

func TestResolverLookupHost(t *testing.T) {
	server := doqServer{}
	server.start()
	defer server.stop()

	resolver := doq.NewResolver(server.addr, doq.WithTLSConfig(generateTLSConfig()))

	addrs, err := resolver.LookupHost(context.Background(), "example.org")

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"127.0.0.1", "::1"}, addrs)
}

func TestDefaultResolver(t *testing.T) {
	server := doqServer{}
	server.start()
	defer server.stop()

	prev := net.DefaultResolver
	net.DefaultResolver = doq.NewResolver(server.addr, doq.WithTLSConfig(generateTLSConfig()))
	t.Cleanup(func() {
		net.DefaultResolver = prev
	})

	addrs, err := net.LookupHost("example.org")

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"127.0.0.1", "::1"}, addrs)
}
