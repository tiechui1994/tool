package rules

import "C"
import (
	"fmt"
	"github.com/tiechui1994/tool/cmd/tcpover/ctx"
	"strconv"
	"strings"
)

type Range struct {
	start, end uint16
}

func NewRange(start, end uint16) *Range {
	return &Range{start: start, end: end}
}

func (r *Range) Contains(v uint16) bool {
	return v >= r.start && v <= r.end
}

type Port struct {
	ruleType string
	portList []Range
	port     string
	adapter  string
}

func NewPort(port string, adapter string, ruleType string) (*Port, error) {
	ports := strings.Split(port, "/")
	if len(ports) > 28 {
		return nil, fmt.Errorf("too many ports to use, maximum support 28 ports")
	}

	var portRange []Range
	for _, p := range ports {
		if p == "" {
			continue
		}

		subPorts := strings.Split(p, "-")
		subPortsLen := len(subPorts)
		if subPortsLen > 2 {
			return nil, errPayload
		}

		portStart, err := strconv.ParseUint(strings.Trim(subPorts[0], "[ ]"), 10, 16)
		if err != nil {
			return nil, errPayload
		}

		switch subPortsLen {
		case 1:
			portRange = append(portRange, *NewRange(uint16(portStart), uint16(portStart)))
		case 2:
			portEnd, err := strconv.ParseUint(strings.Trim(subPorts[1], "[ ]"), 10, 16)
			if err != nil {
				return nil, errPayload
			}
			portRange = append(portRange, *NewRange(uint16(portStart), uint16(portEnd)))
		}
	}

	if len(portRange) == 0 {
		return nil, errPayload
	}

	return &Port{ruleType: ruleType, port: port, portList: portRange, adapter: adapter}, nil
}

func (d *Port) Name() string {
	return d.ruleType
}

func (d *Port) Match(meta *ctx.Metadata) (bool, string) {
	targetPort := meta.DstPort
	switch d.ruleType {
	case RuleSrcPort:
		targetPort = meta.SrcPort
	}

	for _, pr := range d.portList {
		if pr.Contains(targetPort) {
			return true, d.adapter
		}
	}

	return false, d.adapter
}

func (d *Port) Payload() string {
	return d.port
}
