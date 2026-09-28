package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 3 {
		os.Exit(2)
	}
	path, mode := os.Args[1], os.Args[2]
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		panic(err)
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	connection, err := listener.Accept()
	if err != nil {
		panic(err)
	}
	defer connection.Close()
	_, _ = bufio.NewReader(connection).ReadBytes('\n')
	if mode == "protocol" {
		_, _ = fmt.Fprintln(connection, "not-json")
	}
}
