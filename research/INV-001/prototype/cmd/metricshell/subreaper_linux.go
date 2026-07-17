//go:build linux

package main

import "syscall"

const prSetChildSubreaper = 36

func setSubreaper() error {
	_, _, errno := syscall.RawSyscall6(syscall.SYS_PRCTL, prSetChildSubreaper, 1, 0, 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
