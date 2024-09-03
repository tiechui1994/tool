package rules

import (
	"github.com/tiechui1994/tool/cmd/tcpover/transport/ctx"
	"golang.org/x/net/idna"
	"strings"
)

type Domain struct {
	domain string
	adapter string
}

func NewDomain(domain string, adapter string) *Domain {
	punycode, _ := idna.ToASCII(strings.ToLower(domain))
	return &Domain{
		domain:  punycode,
		adapter: adapter,
	}
}

func (d *Domain) Name() string  {
	return RuleDomain
}

func (d *Domain) Match(meta *ctx.Metadata) (bool, string) {
	return d.domain == meta.Host, d.adapter
}

func (d *Domain) Payload() string  {
	return d.domain
}

