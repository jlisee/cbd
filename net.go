package cbd

import (
	"fmt"
	"log"
	"math/big"
	"net"
	"sort"
	"strings"
)

// Custom IP sortring (doesn't support IPv6), which puts private IP addresses
// first, 192, then 172, then 10, then public
type ByPrivateIPAddr []net.IPNet

func (a ByPrivateIPAddr) Len() int {
	return len(a)
}
func (a ByPrivateIPAddr) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}
func (a ByPrivateIPAddr) Less(i, j int) bool {

	l := big.NewInt(0)
	l.SetBytes(a[i].IP)

	r := big.NewInt(0)
	r.SetBytes(a[j].IP)

	cmpRes := l.Cmp(r) == -1

	if a[i].IP.To4() != nil && a[j].IP.To4() != nil {

		fidx := 0

		switch len(a[i].IP) {
		case net.IPv4len:
			fidx = 0
		case net.IPv6len:
			fidx = 12
		default:
			return false
		}

		fl := a[i].IP[fidx]
		fr := a[j].IP[fidx]

		if fl == fr {
			return cmpRes
		}

		switch fl {

		case 192:
			return true

		case 172:
			switch fr {
			case 10:
				return true
			case 192:
				return false
			default:
				return true
			}

		case 10:
			switch fr {
			case 192:
				return false
			case 172:
				return false
			default:
				return true
			}

		default:

			switch fr {
			case 192:
				fallthrough
			case 172:
				fallthrough
			case 10:
				return false
			default:
				return cmpRes
			}
		}

	} else {
		return cmpRes
	}
}

// Returns all local IPv4 addresses (besides loop back), along with mask
func getLocalIPAddrs() (addrs []net.IPNet, err error) {

	ifaces, err := net.Interfaces()

	if err != nil {
		return
	}

	for _, iface := range ifaces {

		var iaddrs []net.Addr
		iaddrs, err = iface.Addrs()

		if err != nil {
			break
		}

		for _, iaddr := range iaddrs {
			// Basic parse for the IP
			as := iaddr.String()

			_, addr, err := net.ParseCIDR(as)

			if err != nil {
				log.Print("Error parsing address: ", as)
				continue
			}

			// Now update the IP address (the CIDR mask out the lower bits that
			// we want to keep around)
			parts := strings.Split(as, "/")

			ip := net.ParseIP(parts[0])

			if ip == nil {
				log.Print("Error parsing IP: ", as)
				continue

			}

			addr.IP = ip

			// Ignore local and IPv6 addresses
			if addr.IP.IsLoopback() || addr.IP.To4() == nil {
				continue
			}

			// Apend to our list
			addrs = append(addrs, *addr)
		}
	}

	// Sort our lists of addresses
	sort.Sort(ByPrivateIPAddr(addrs))

	return
}

// Returns the first IP from the b list which is contained in one of the network
// interfaces listed in the a list
func getMatchingIP(as []net.IPNet, bs []net.IPNet) (net.IPNet, error) {
	for _, a := range as {
		for _, b := range bs {
			if a.Contains(b.IP) {
				return b, nil
			}
		}
	}

	var ret net.IPNet
	return ret, fmt.Errorf("Count not find matching IP")
}
