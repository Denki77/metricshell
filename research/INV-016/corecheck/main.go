package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Denki77/metricshell/implementation/internal/snapshot"
)

func main() {
	if len(os.Args) < 2 {
		panic("snapshot path required")
	}
	fmt.Println("file\texpected\tactual\tseries\tcanonical_bytes\tresult")
	failed := false
	for _, argument := range os.Args[1:] {
		path, expected, ok := strings.Cut(argument, "=")
		if !ok {
			expected = "pass"
		}
		content, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("%s\t%s\tread_error:%s\t-\t-\tfail\n", path, expected, err)
			failed = true
			continue
		}
		validated, err := snapshot.Parse(content, snapshot.DefaultLimits())
		if err != nil {
			actual := "reject:" + err.Error()
			result := "pass"
			if actual != expected {
				result = "fail"
				failed = true
			}
			fmt.Printf("%s\t%s\t%s\t-\t-\t%s\n", path, expected, actual, result)
			continue
		}
		result := "pass"
		if expected != "pass" {
			result = "fail"
			failed = true
		}
		fmt.Printf("%s\t%s\tpass\t%d\t%d\t%s\n", path, expected, validated.SeriesCount(), validated.CanonicalBytes(), result)
	}
	if failed {
		os.Exit(1)
	}
}
