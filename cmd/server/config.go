package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

func validateAddress(addr string) error {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("监听地址格式无效: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("监听地址必须使用明确的回环 IP")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1024 || port > 65535 {
		return fmt.Errorf("监听端口必须是 1024-65535 的高位端口")
	}
	return nil
}
func addressFromPort(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1024 || port > 65535 {
		return "", fmt.Errorf("PORT 必须是 1024-65535 的端口号")
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), nil
}
