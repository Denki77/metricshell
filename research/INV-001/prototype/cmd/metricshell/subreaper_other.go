//go:build !linux

package main

import "errors"

func setSubreaper() error {
	return errors.New("PR_SET_CHILD_SUBREAPER is only available on Linux")
}
