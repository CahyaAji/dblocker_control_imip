// UDP command sender for DBlocker Master
// Turns on master[6] and slave[6] (bit 6 and bit 13)
// Usage: go run send_udp_mask.go

package main

import (
	"fmt"
	"net"
	"os"
)

func crc8(data string) byte {
	crc := byte(0)
	for i := 0; i < len(data); i++ {
		crc ^= data[i]
	}
	return crc
}

func main() {
	const (
		udpSecret = "p!ml_3rUc35"
		udpPort   = 51515        // Change if needed
		masterIP  = "10.88.81.3" // Change if needed
	)
	// 0,0,0,0,0,0,1 (bit 6) for master, 0,0,0,0,0,0,1 (bit 13) for slave
	mask := 0x2040 // 0b0010_0000_0100_0000
	payload := fmt.Sprintf("%s:MASK:%04X", udpSecret, mask)
	crc := crc8(payload)
	packet := fmt.Sprintf("%s|%02X", payload, crc)

	conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", masterIP, udpPort))
	if err != nil {
		fmt.Println("Dial error:", err)
		os.Exit(1)
	}
	defer conn.Close()

	_, err = conn.Write([]byte(packet))
	if err != nil {
		fmt.Println("Write error:", err)
		os.Exit(1)
	}
	fmt.Println("Sent:", packet)
}
