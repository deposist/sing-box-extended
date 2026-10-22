package server

import (
	CM "github.com/sagernet/sing-box/service/manager/constant"
	pb "github.com/sagernet/sing-box/service/node_manager_api/manager"
)

// convertHello translates the wire self-description into the manager's own
// shape. A nil hello (an older node) converts to the zero value, which the
// manager treats as "nothing announced".
func convertHello(hello *pb.NodeHello) CM.NodeHello {
	if hello == nil {
		return CM.NodeHello{}
	}
	converted := CM.NodeHello{
		ProtocolVersion: hello.GetProtocolVersion(),
		CoreVersion:     hello.GetCoreVersion(),
	}
	if inbounds := hello.GetInbounds(); len(inbounds) > 0 {
		converted.Inbounds = make([]CM.NodeHelloInbound, len(inbounds))
		for i, inbound := range inbounds {
			converted.Inbounds[i] = CM.NodeHelloInbound{Tag: inbound.GetTag(), Type: inbound.GetType()}
		}
	}
	if limiters := hello.GetLimiters(); len(limiters) > 0 {
		converted.Limiters = make([]CM.NodeHelloLimiter, len(limiters))
		for i, limiter := range limiters {
			converted.Limiters[i] = CM.NodeHelloLimiter{
				Tag:     limiter.GetTag(),
				Type:    limiter.GetType(),
				Variant: limiter.GetVariant(),
			}
		}
	}
	return converted
}
