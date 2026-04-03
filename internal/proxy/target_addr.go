package proxy

import (
	"fmt"
	"net"
	"strconv"
)

const (
	socks5AddrTypeIPv4   = 0x01
	socks5AddrTypeDomain = 0x03
	socks5AddrTypeIPv6   = 0x04
)

type TargetAddr struct {
	Host string
	Port uint16
	IP   net.IP
}

func (t TargetAddr) String() string {
	if t.IP != nil {
		return net.JoinHostPort(t.IP.String(), strconv.Itoa(int(t.Port)))
	}
	return net.JoinHostPort(t.Host, strconv.Itoa(int(t.Port)))
}

func (t TargetAddr) ToBytes() ([]byte, error) {
	if ip4 := t.IP.To4(); ip4 != nil {
		out := []byte{socks5AddrTypeIPv4}
		out = append(out, ip4...)
		out = append(out, byte(t.Port>>8), byte(t.Port))
		return out, nil
	}
	if ip16 := t.IP.To16(); ip16 != nil && t.IP != nil {
		out := []byte{socks5AddrTypeIPv6}
		out = append(out, ip16...)
		out = append(out, byte(t.Port>>8), byte(t.Port))
		return out, nil
	}
	if len(t.Host) > 255 {
		return nil, fmt.Errorf("域名长度超限")
	}
	out := []byte{socks5AddrTypeDomain, byte(len(t.Host))}
	out = append(out, []byte(t.Host)...)
	out = append(out, byte(t.Port>>8), byte(t.Port))
	return out, nil
}

func ReadTargetAddr(data []byte, atyp byte) (TargetAddr, int, bool, error) {
	switch atyp {
	case socks5AddrTypeIPv4:
		if len(data) < 6 {
			return TargetAddr{}, 0, false, nil
		}
		ip := net.IPv4(data[0], data[1], data[2], data[3])
		port := uint16(data[4])<<8 | uint16(data[5])
		return TargetAddr{IP: ip, Port: port}, 6, true, nil
	case socks5AddrTypeIPv6:
		if len(data) < 18 {
			return TargetAddr{}, 0, false, nil
		}
		ip := net.IP(append([]byte(nil), data[:16]...))
		port := uint16(data[16])<<8 | uint16(data[17])
		return TargetAddr{IP: ip, Port: port}, 18, true, nil
	case socks5AddrTypeDomain:
		if len(data) < 2 {
			return TargetAddr{}, 0, false, nil
		}
		size := int(data[0])
		if len(data) < 1+size+2 {
			return TargetAddr{}, 0, false, nil
		}
		host := string(data[1 : 1+size])
		portIdx := 1 + size
		port := uint16(data[portIdx])<<8 | uint16(data[portIdx+1])
		if parsed := net.ParseIP(host); parsed != nil {
			return TargetAddr{IP: parsed, Port: port}, 1 + size + 2, true, nil
		}
		return TargetAddr{Host: host, Port: port}, 1 + size + 2, true, nil
	default:
		return TargetAddr{}, 0, false, fmt.Errorf("不支持的地址类型")
	}
}
