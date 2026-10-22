package constant

// NodeHello is the manager-side view of what a node announced when it
// connected: the protocol version it speaks and the tags it can accept users
// and limiters for. Recording it is the manager's job; validating a rollout
// against it is the caller's.
type NodeHello struct {
	ProtocolVersion uint32
	CoreVersion     string
	Inbounds        []NodeHelloInbound
	Limiters        []NodeHelloLimiter
}

type NodeHelloInbound struct {
	Tag  string
	Type string
	// Descriptor is an opaque, node-defined blob describing how to build a
	// client link for this inbound. The core never reads it; it is stored so a
	// controller can assemble a subscription covering every node a client
	// reaches. Only the embedding panel knows the format.
	Descriptor string
}

type NodeHelloLimiter struct {
	Tag     string
	Type    string
	Variant string
}
