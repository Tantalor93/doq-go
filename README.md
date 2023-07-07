# doq-go
[![Tantalor93](https://circleci.com/gh/Tantalor93/doq-go/tree/main.svg?style=svg)](https://circleci.com/gh/Tantalor93/doq-go?branch=main)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

DoQ client library written in Golang built on top [quic-go](https://github.com/quic-go/quic-go) and [dns](https://github.com/miekg/dns)
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
