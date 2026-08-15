//go:build !linux

package workload

import "errors"

func enableSubreaper() error {
	return errors.New("child subreaper is unsupported on this platform")
}
