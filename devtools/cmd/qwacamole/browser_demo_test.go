package main

import (
	"net"
	"strings"
	"testing"
)

func TestPortPreflightReleasesReservationsOnSuccessAndFailure(t *testing.T) {
	// Real OS sockets prove a failed later check cannot retain an earlier port
	// and block the user's next attempt. No actual demo service is involved.
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, freePort, _ := net.SplitHostPort(free.Addr().String())
	free.Close()
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, udpPort, _ := net.SplitHostPort(udp.LocalAddr().String())
	udp.Close()
	ports := []demoPort{{"tcp", freePort}, {"udp", udpPort}}
	if err = checkPorts(ports); err != nil {
		t.Fatal(err)
	}
	blocked, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer blocked.Close()
	_, blockedPort, _ := net.SplitHostPort(blocked.Addr().String())
	if err = checkPorts(append(ports, demoPort{"tcp", blockedPort})); err == nil || !strings.Contains(err.Error(), blockedPort) {
		t.Fatal("missing intended port-conflict failure", err)
	}
	tcpAgain, err := net.Listen("tcp", "127.0.0.1:"+freePort)
	if err != nil {
		t.Fatal("preflight retained TCP reservation", err)
	}
	tcpAgain.Close()
	udpAgain, err := net.ListenPacket("udp", "127.0.0.1:"+udpPort)
	if err != nil {
		t.Fatal("preflight retained UDP reservation", err)
	}
	udpAgain.Close()
}
