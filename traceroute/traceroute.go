// TODO: support ipv6
// Package traceroute provides functions for executing a tracroute to a remote
// host.
package traceroute

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
)

// Return the first non-loopback address as a 4 byte IP address. This address
// is used for sending packets out.
func getLocalIP() (net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			return ipnet.IP, nil
		}
	}
	return nil, errors.New("You do not appear to be connected to the Internet")
}

// Given a host name convert it to a 4 byte IP address.
func destAddr(dest string) (net.IP, error) {
	ips, err := net.LookupIP(dest)
	if err != nil {
		return nil, err
	}
	ip := ips[0]
	return ip, nil

	/*
		ipAddr, err := net.ResolveIPAddr("ip", addr)
		if err != nil {
			return
		}
		copy(destAddr[:], ipAddr.IP.To4())
		return
	*/
}

// TracerouteHop type
type TracerouteHop struct {
	Success     bool
	Address     net.IP
	N           int
	ElapsedTime time.Duration
	TTL         int
}

func (hop *TracerouteHop) AddressString() string {
	return fmt.Sprintf("%v.%v.%v.%v", hop.Address[0], hop.Address[1], hop.Address[2], hop.Address[3])
}

// TracerouteResult type
type TracerouteResult struct {
	DestinationAddress net.IP
	Hops               []TracerouteHop
}

func notify(hop TracerouteHop, channels []chan TracerouteHop) {
	for _, c := range channels {
		c <- hop
	}
}

func closeNotify(channels []chan TracerouteHop) {
	for _, c := range channels {
		close(c)
	}
}

func ipToSockaddr(ip net.IP, port int) syscall.Sockaddr {
	if ip4 := ip.To4(); ip4 != nil {
		var ip [4]byte
		copy(ip[:], ip4)
		return &syscall.SockaddrInet4{
			Port: port,
			Addr: ip,
		}
	} else if ip6 := ip.To16(); ip6 != nil {
		var ip [16]byte
		copy(ip[:], ip6)
		return &syscall.SockaddrInet6{
			Port: port,
			Addr: ip,
		}
	}
	return nil
}

// Traceroute uses the given dest (hostname) and options to execute a traceroute
// from your machine to the remote host.
//
// Outbound packets are UDP packets and inbound packets are ICMP.
//
// Returns a TracerouteResult which contains an array of hops. Each hop includes
// the elapsed time and its IP address.
func Traceroute(dest string, options *TracerouteOptions, c ...chan TracerouteHop) (result TracerouteResult, err error) {
	result.Hops = []TracerouteHop{}
	destIP, err := destAddr(dest)
	result.DestinationAddress = destIP
	sourceIP, err := getLocalIP()
	if err != nil {
		return
	}

	// Convert the net.IPs to the appropriate sockaddrs
	destSockaddr := ipToSockaddr(destIP, options.DstPort())
	sourceSockaddr := ipToSockaddr(sourceIP, options.SrcPort())

	timeoutMs := (int64)(options.TimeoutMs())
	tv := syscall.NsecToTimeval(1000 * 1000 * timeoutMs)

	ttl := 1
	retry := 0
	for {
		//logrus.Infof("dst: %s TTL: %v", dest, ttl)
		start := time.Now()

		// Set up the socket to receive inbound packets
		recvSocket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_ICMP)
		if err != nil {
			return result, err
		}

		// Set up the socket to send packets out.
		sendSocket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
		if err != nil {
			return result, err
		}
		// This sets the current hop TTL
		syscall.SetsockoptInt(sendSocket, 0x0, syscall.IP_TTL, ttl)
		// This sets the timeout to wait for a response from the remote host
		syscall.SetsockoptTimeval(recvSocket, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)

		defer syscall.Close(recvSocket)
		defer syscall.Close(sendSocket)

		// Bind to the local socket to listen for ICMP packets
		syscall.Bind(recvSocket, &syscall.SockaddrInet4{Port: options.DstPort()})

		if options.SrcPort() > 0 {
			// if srcPort is set, bind to that as well
			syscall.SetsockoptInt(sendSocket, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			err = syscall.Bind(sendSocket, sourceSockaddr)
			// TODO: non-fatal error
			if err != nil {
				return result, err
			}
		}

		// Send a single null byte UDP packet
		syscall.Sendto(sendSocket, []byte{0x0}, 0, destSockaddr)

		var p = make([]byte, options.PacketSize())
		n, from, err := syscall.Recvfrom(recvSocket, p, 0)
		elapsed := time.Since(start)
		if err == nil {
			var currIP net.IP
			switch t := from.(type) {
			case *syscall.SockaddrInet4:
				currIP = net.IP(t.Addr[:])
			case *syscall.SockaddrInet6:
				currIP = net.IP(t.Addr[:])
			}

			hop := TracerouteHop{
				Success:     true,
				Address:     currIP,
				N:           n,
				ElapsedTime: elapsed,
				TTL:         ttl,
			}

			notify(hop, c)

			result.Hops = append(result.Hops, hop)

			ttl += 1
			retry = 0

			if ttl > options.MaxHops() || currIP.Equal(destIP) {
				closeNotify(c)
				return result, nil
			}
		} else {
			retry += 1
			if retry > options.Retries() {
				notify(TracerouteHop{Success: false, TTL: ttl}, c)
				ttl += 1
				retry = 0
			}

			if ttl > options.MaxHops() {
				closeNotify(c)
				return result, nil
			}
		}

	}
}
