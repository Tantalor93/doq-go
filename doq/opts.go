package doq

import (
	"crypto/tls"
	"time"
)

// Option represents configuration options for doq.Client.
type Option interface {
	apply(c *Client)
}

type tlsConfigOption struct {
	tlsConfig *tls.Config
}

func (o *tlsConfigOption) apply(c *Client) {
	c.tlsConfig = o.tlsConfig.Clone()
}

// WithTLSConfig is a configuration option that sets TLS configuration for the doq.Client.
func WithTLSConfig(c *tls.Config) Option {
	return &tlsConfigOption{
		tlsConfig: c,
	}
}

type writeTimeoutOption struct {
	writeTimeout time.Duration
}

func (o *writeTimeoutOption) apply(c *Client) {
	c.writeTimeout = o.writeTimeout
}

// WithWriteTimeout is a configuration option that sets write timeout for the doq.Client.
func WithWriteTimeout(t time.Duration) Option {
	return &writeTimeoutOption{
		writeTimeout: t,
	}
}

type readTimeoutOption struct {
	readTimeout time.Duration
}

func (o *readTimeoutOption) apply(c *Client) {
	c.readTimeout = o.readTimeout
}

// WithReadTimeout is a configuration option that sets read timeout for the doq.Client.
func WithReadTimeout(t time.Duration) Option {
	return &readTimeoutOption{
		readTimeout: t,
	}
}

type connectTimeoutOption struct {
	connectTimeout time.Duration
}

func (o *connectTimeoutOption) apply(c *Client) {
	c.connectTimeout = o.connectTimeout
}

// WithConnectTimeout is a configuration option that sets connect timeout for the doq.Client.
func WithConnectTimeout(t time.Duration) Option {
	return &connectTimeoutOption{
		connectTimeout: t,
	}
}
