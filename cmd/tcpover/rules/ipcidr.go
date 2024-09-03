package rules

import (
	"github.com/tiechui1994/tool/cmd/tcpover/transport/common/netip"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/ctx"
)

type IPCIDR struct {
	ipnet   *netip.Prefix
	adapter string
}

func NewIPCIDR(s string, adapter string) (*IPCIDR, error) {
	ipnet, err := netip.ParsePrefix(s)
	if err != nil {
		return nil, err
	}

	return &IPCIDR{
		ipnet:   &ipnet,
		adapter: adapter,
	}, nil
}

func (d *IPCIDR) Name() string {
	return RuleIPCIDR
}

func (d *IPCIDR) Match(meta *ctx.Metadata) (bool, string) {
	ip := meta.DstIP
	return d.ipnet.Contains(netip.MustParseAddr(ip.String())), d.adapter
}

func (d *IPCIDR) Payload() string {
	return d.ipnet.String()
}
