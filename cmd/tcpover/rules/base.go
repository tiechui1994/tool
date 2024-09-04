package rules

import (
	"errors"
	"github.com/tiechui1994/tool/cmd/tcpover/ctx"
)

var (
	errPayload = errors.New("payloadRule error")
)

type Rule interface {
	Name() string
	Match(meta *ctx.Metadata) (bool, string)
	Payload() string
}

const (
	RuleDomain        = "DOMAIN"
	RuleDomainKeyword = "DOMAIN-KEYWORD"
	RuleDomainSuffix  = "DOMAIN-SUFFIX"
	RuleIPCIDR        = "IPCIDR"
	RuleMatch         = "MATCH"
	RuleDstPort       = "DST-PORT"
	RuleSrcPort       = "SRC-PORT"
)
