package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	fmt.Printf("metricshell research version=%s arch=%s os=%s uid=%d gid=%d\n", version, runtime.GOARCH, runtime.GOOS, os.Getuid(), os.Getgid())
}
