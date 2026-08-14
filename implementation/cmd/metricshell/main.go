package main

import (
	"os"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/buildinfo"
	"github.com/Denki77/metricshell/implementation/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, buildinfo.Current(), time.Now))
}
