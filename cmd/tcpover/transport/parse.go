package transport

import (
	"fmt"
	"github.com/tiechui1994/tool/cmd/tcpover/ctx"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/common/structure"
	"github.com/tiechui1994/tool/cmd/tcpover/transport/outbound"
)

func ParseProxy(mapping map[string]interface{}) (ctx.Proxy, error) {
	decoder := structure.NewDecoder(structure.Option{TagName: "proxy", WeaklyTypedInput: true, KeyReplacer: structure.DefaultKeyReplacer})
	proxyType, existType := mapping["type"].(string)
	if !existType {
		return nil, fmt.Errorf("missing type")
	}
	var (
		proxy ctx.Proxy
		err   error
	)
	switch proxyType {
	case ctx.Wless:
		muxOption := &outbound.WebSocketOption{}
		err = decoder.Decode(mapping, muxOption)
		if err != nil {
			break
		}
		proxy, err = outbound.NewWless(*muxOption)
	case ctx.Direct:
		proxy = outbound.NewDirect()
	default:
		return nil, fmt.Errorf("unsupport proxy type: %s", proxyType)
	}

	return proxy, nil
}
