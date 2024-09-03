package rules

import (
	"github.com/tiechui1994/tool/cmd/tcpover/transport/ctx"
)

type Match struct {
	adapter string
}

func NewMatch(adapter string) *Match {
	return &Match{adapter: adapter}
}

func (d *Match) Name() string {
	return RuleMATCH
}

func (d *Match) Match(meta *ctx.Metadata) (bool, string) {
	return true, d.adapter
}

func (d *Match) Payload() string {
	return ""
}
