// Command faketunnel impersonates a wildcard-bound tunnel for the exit-code
// integration tests. It is built into a file named after one of tunnelsnoop's
// allowed binaries (e.g. "ssh"), which is what /proc/<pid>/comm reports and
// therefore what discovery matches on.
//
// It lives under testdata so that the go tool skips it when expanding ./...;
// the tests build it explicitly.
package main

import (
	"fmt"
	"net"
	"os"
)

func main() {
	// Bind the wildcard address so the listener is discovered as exposed.
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "faketunnel: %v\n", err)
		os.Exit(1)
	}

	// Report the kernel-assigned port so the test can scope tunnelsnoop to
	// this listener with -port and ignore any real tunnel on the host.
	fmt.Println(listener.Addr().(*net.TCPAddr).Port)

	// Serve rather than sleep: a process blocked in accept keeps the socket in
	// LISTEN, whereas a fully idle Go program is killed by the deadlock
	// detector.
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		_ = conn.Close()
	}
}
