//go:build linux

package fileingest

import (
	"context"
	"errors"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

const watchMask = syscall.IN_CREATE | syscall.IN_MOVED_TO | syscall.IN_CLOSE_WRITE | syscall.IN_DELETE | syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF

func (reconciler *Reconciler) Run(ctx context.Context) error {
	reconciler.Reconcile(ctx, Startup)
	descriptor, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return err
	}
	defer syscall.Close(descriptor)

	directory, target := filepath.Split(reconciler.configuration.Path)
	watch := -1
	install := func() bool {
		if watch >= 0 {
			return true
		}
		installed, watchErr := syscall.InotifyAddWatch(descriptor, filepath.Clean(directory), watchMask)
		if watchErr != nil {
			return false
		}
		watch = installed
		return true
	}
	install()
	ticker := time.NewTicker(reconciler.configuration.ReconcileInterval)
	defer ticker.Stop()
	buffer := make([]byte, 16<<10)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if watch < 0 && install() {
				reconciler.observer.WatchEvent("reinstalled")
				reconciler.Reconcile(ctx, WatchReinstall)
			}
			reconciler.Reconcile(ctx, Periodic)
		default:
			count, readErr := syscall.Read(descriptor, buffer)
			if readErr != nil {
				if errors.Is(readErr, syscall.EAGAIN) || errors.Is(readErr, syscall.EINTR) {
					time.Sleep(time.Millisecond)
					continue
				}
				return readErr
			}
			for offset := 0; offset+syscall.SizeofInotifyEvent <= count; {
				event := (*syscall.InotifyEvent)(unsafe.Pointer(&buffer[offset]))
				name := eventName(buffer[offset+syscall.SizeofInotifyEvent : offset+syscall.SizeofInotifyEvent+int(event.Len)])
				offset += syscall.SizeofInotifyEvent + int(event.Len)
				if event.Mask&syscall.IN_Q_OVERFLOW != 0 {
					reconciler.observer.WatchEvent("overflow")
					reconciler.Reconcile(ctx, Overflow)
					continue
				}
				if event.Mask&(syscall.IN_IGNORED|syscall.IN_DELETE_SELF|syscall.IN_MOVE_SELF) != 0 {
					watch = -1
					reconciler.observer.WatchEvent("invalidated")
					continue
				}
				if name == target && event.Mask&watchMask != 0 {
					reconciler.Reconcile(ctx, Event)
				}
			}
		}
	}
}

func eventName(encoded []byte) string {
	for index, value := range encoded {
		if value == 0 {
			return string(encoded[:index])
		}
	}
	return string(encoded)
}
