package main

import (
	"bytes"
	"fmt"
	"net"

	"github.com/hashicorp/go-msgpack/codec"
)

// Decode reverses the encode operation on a byte slice input
func decode(buf []byte, out interface{}) error {
	r := bytes.NewReader(buf)
	hd := codec.MsgpackHandle{}
	dec := codec.NewDecoder(r, &hd)
	return dec.Decode(out)
}

// Encode writes an encoded object to a new bytes buffer
func encode(msgType messageType, in interface{}) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte(uint8(msgType))
	hd := codec.MsgpackHandle{}
	enc := codec.NewEncoder(buf, &hd)
	err := enc.Encode(in)
	return buf, err
}

// for a given interface name, return the first net.IP we find
func ipForInterface(name string) (net.IP, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	for _, rawAddr := range addrs {
		var ip net.IP
		switch addr := rawAddr.(type) {
		case *net.IPAddr:
			ip = addr.IP
		case *net.IPNet:
			ip = addr.IP
		default:
			continue
		}
		// TODO: skip if private?
		return net.IP(ip), nil
	}
	return nil, fmt.Errorf("Unable to find ip for interface %s", name)
}

// GetLocalIP returns the non loopback local IP of the host
func GetLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("Unable to find local non-loopback address")
}
