# doq-go
[![Tantalor93](https://circleci.com/gh/Tantalor93/doq-go/tree/main.svg?style=svg)](https://circleci.com/gh/Tantalor93/doq-go?branch=main)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![lint](https://github.com/Tantalor93/doq-go/actions/workflows/lint.yml/badge.svg?branch=main)](https://github.com/Tantalor93/doq-go/actions/workflows/lint.yml)
[![codecov](https://codecov.io/gh/Tantalor93/doq-go/branch/main/graph/badge.svg?token=77659YBXM8)](https://codecov.io/gh/Tantalor93/doq-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/tantalor93/doq-go)](https://goreportcard.com/report/github.com/tantalor93/doq-go)
[![](https://godoc.org/github.com/Tantalor93/doq-go/doq?status.svg)](https://godoc.org/github.com/tantalor93/doq-go/doq)

DNS over QUIC (=DoQ, as defined in [RFC9250](https://datatracker.ietf.org/doc/rfc9250/)) client library written in Golang and built on top [quic-go](https://github.com/quic-go/quic-go) and [dns](https://github.com/miekg/dns)
libraries.

## Usage in your project
add dependency
```
go get github.com/tantalor93/doq-go
```

## Examples
```
// create new DoQ Client
client, err := doq.NewClient("dns.adguard-dns.com:853", doq.Options{})
if err != nil {
    panic(err)
}

// create new query
q := dns.Msg{}
q.SetQuestion("www.google.com.", dns.TypeA)

// send query
var resp *dns.Msg
resp, err = client.Send(context.Background(), &q)
fmt.Println(resp ,err)
```
