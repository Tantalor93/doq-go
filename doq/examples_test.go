package doq_test

import (
	"context"
	"fmt"

	"github.com/miekg/dns"
	"github.com/tantalor93/doq-go/doq"
)

func ExampleClient_Send() {
	// create client with default settings resolving via AdGuard DoQ Server
	client := doq.NewClient("dns.adguard-dns.com:853")

	// prepare payload
	q := dns.Msg{}
	q.SetQuestion("www.google.com.", dns.TypeA)

	// send DNS query
	r, err := client.Send(context.Background(), &q)
	if err != nil {
		panic(err)
	}
	// do something with response
	fmt.Println(dns.RcodeToString[r.Rcode])
	// Output: NOERROR
}
