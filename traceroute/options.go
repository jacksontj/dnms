package traceroute // TracrouteOptions type

const DEFAULT_PORT = 33434
const DEFAULT_MAX_HOPS = 64
const DEFAULT_TIMEOUT_MS = 500
const DEFAULT_RETRIES = 3
const DEFAULT_PACKET_SIZE = 52

type TracerouteOptions struct {
	dstPort    int
	srcPort    int
	maxHops    int
	timeoutMs  int
	retries    int
	packetSize int
}

func (options *TracerouteOptions) DstPort() int {
	if options.dstPort == 0 {
		options.dstPort = DEFAULT_PORT
	}
	return options.dstPort
}

func (options *TracerouteOptions) SetDstPort(port int) {
	options.dstPort = port
}

func (options *TracerouteOptions) SrcPort() int {
	return options.srcPort
}

func (options *TracerouteOptions) SetSrcPort(port int) {
	options.srcPort = port
}

func (options *TracerouteOptions) MaxHops() int {
	if options.maxHops == 0 {
		options.maxHops = DEFAULT_MAX_HOPS
	}
	return options.maxHops
}

func (options *TracerouteOptions) SetMaxHops(maxHops int) {
	options.maxHops = maxHops
}

func (options *TracerouteOptions) TimeoutMs() int {
	if options.timeoutMs == 0 {
		options.timeoutMs = DEFAULT_TIMEOUT_MS
	}
	return options.timeoutMs
}

func (options *TracerouteOptions) SetTimeoutMs(timeoutMs int) {
	options.timeoutMs = timeoutMs
}

func (options *TracerouteOptions) Retries() int {
	if options.retries == 0 {
		options.retries = DEFAULT_RETRIES
	}
	return options.retries
}

func (options *TracerouteOptions) SetRetries(retries int) {
	options.retries = retries
}

func (options *TracerouteOptions) PacketSize() int {
	if options.packetSize == 0 {
		options.packetSize = DEFAULT_PACKET_SIZE
	}
	return options.packetSize
}

func (options *TracerouteOptions) SetPacketSize(packetSize int) {
	options.packetSize = packetSize
}
