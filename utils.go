package net

import (
	"fmt"
	"net"
	"strings"

	"github.com/prestonTao/upnp"
	"github.com/sirupsen/logrus"
)

// GetAddresses -
func GetAddresses(port int) ([]string, error) {
	// add all addresses to peer
	ips := []string{}

	// go through all ifs
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	// and find their addresses
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			addr := ""
			if strings.Contains(ip.String(), ":") {
				ips = append(ips, fmt.Sprintf("tcp6:[%s]:%d", ip.String(), port))
				ips = append(ips, fmt.Sprintf("udp6:[%s]:%d", ip.String(), port))
			} else {
				ips = append(ips, fmt.Sprintf("tcp4:%s:%d", ip.String(), port))
				ips = append(ips, fmt.Sprintf("udp4:%s:%d", ip.String(), port))
			}
			if addr == "" {
				continue
			}
			ips = append(ips, addr)
		}
	}

	if eip, err := getPublicAddress(); err != nil {
		logrus.
			WithError(err).
			Warnf("Could not get external IP")
	} else {
		ips = append(ips, fmt.Sprintf("tcp4:%s:%d", eip, port))
		ips = append(ips, fmt.Sprintf("udp4:%s:%d", eip, port))
	}

	return ips, nil
}

func getPublicAddress() (string, error) {
	// TODO Better logging
	upnpMan := new(upnp.Upnp)
	err := upnpMan.ExternalIPAddr()
	if err != nil {
		return "", err
	}

	return upnpMan.GatewayOutsideIP, nil
}

