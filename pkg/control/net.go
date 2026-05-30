package control

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

// DefaultPort is the canonical control-server port shown on the splash. It's the
// port a parent may already know from the flashcard "open on your phone" flow.
const DefaultPort = 8080

// Port is the control server port: $WAND_CONTROL_PORT, else DefaultPort.
func Port() int {
	if p := os.Getenv("WAND_CONTROL_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 && n < 65536 {
			return n
		}
	}
	return DefaultPort
}

// LocalIP returns the host's primary LAN IPv4 address (best effort) by checking
// which local interface would reach the internet. No packets are sent. Returns
// "" if it can't be determined.
func LocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return addr.IP.String()
	}
	return ""
}

// URL is the connect URL shown on the splash, e.g. "http://10.0.0.5:8080",
// falling back to localhost when the LAN IP can't be determined.
func URL(port int) string {
	host := LocalIP()
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}
