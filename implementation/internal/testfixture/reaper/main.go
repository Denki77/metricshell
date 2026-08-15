package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

const (
	readyFD   = 3
	burstSize = 64
)

func main() {
	if len(os.Args) > 1 {
		runRole(os.Args[1:])
		return
	}

	switch os.Getenv("REAPER_MODE") {
	case "primary-before-child":
		launchOrphans(1, 250*time.Millisecond)
		os.Exit(21)
	case "child-before-primary":
		launchOrphans(1, 10*time.Millisecond)
		time.Sleep(150 * time.Millisecond)
		os.Exit(22)
	case "burst":
		launchOrphans(burstSize, 10*time.Millisecond)
		zombies := waitForStableZombieCount(2 * time.Second)
		if err := writeJSON(map[string]any{"event": "fixture.audit", "zombies": zombies}); err != nil {
			os.Exit(70)
		}
		if zombies != 0 {
			os.Exit(24)
		}
		os.Exit(23)
	default:
		if _, err := fmt.Fprintln(os.Stderr, "REAPER_MODE is required"); err != nil {
			os.Exit(70)
		}
		os.Exit(64)
	}
}

func runRole(args []string) {
	switch args[0] {
	case "launcher":
		if len(args) != 2 {
			os.Exit(64)
		}
		command := exec.Command(os.Args[0], "leaf", args[1])
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Start(); err != nil {
			os.Exit(70)
		}
		ready := os.NewFile(readyFD, "ready")
		if ready == nil {
			os.Exit(70)
		}
		if _, err := ready.Write([]byte{1}); err != nil {
			os.Exit(70)
		}
		_ = ready.Close()
	case "leaf":
		if len(args) != 2 {
			os.Exit(64)
		}
		delay, err := time.ParseDuration(args[1])
		if err != nil {
			os.Exit(64)
		}
		time.Sleep(delay)
	default:
		os.Exit(64)
	}
}

func launchOrphans(count int, leafDelay time.Duration) {
	reader, writer, err := os.Pipe()
	if err != nil {
		os.Exit(70)
	}
	commands := make([]*exec.Cmd, 0, count)
	for range count {
		command := exec.Command(os.Args[0], "launcher", leafDelay.String())
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		command.ExtraFiles = []*os.File{writer}
		if err := command.Start(); err != nil {
			os.Exit(70)
		}
		commands = append(commands, command)
	}
	_ = writer.Close()

	ready := make([]byte, count)
	read := 0
	for read < len(ready) {
		amount, err := reader.Read(ready[read:])
		if err != nil {
			os.Exit(70)
		}
		read += amount
	}
	_ = reader.Close()
	for _, command := range commands {
		if err := command.Wait(); err != nil {
			os.Exit(70)
		}
	}
}

func waitForStableZombieCount(timeout time.Duration) int {
	deadline := time.Now().Add(timeout)
	supervisorPID := os.Getppid()
	stable := 0
	last := -1
	for time.Now().Before(deadline) {
		current := adoptedZombies(supervisorPID)
		if current == 0 && last == 0 {
			stable++
			if stable == 3 {
				return 0
			}
		} else {
			stable = 0
		}
		last = current
		time.Sleep(20 * time.Millisecond)
	}
	return adoptedZombies(supervisorPID)
}

func adoptedZombies(supervisorPID int) int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return -1
	}
	supervisor := strconv.Itoa(supervisorPID)
	zombies := 0
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		content, err := os.ReadFile("/proc/" + entry.Name() + "/stat")
		if err != nil {
			continue
		}
		fields := bytes.Fields(content[bytes.LastIndexByte(content, ')')+1:])
		if len(fields) >= 2 && string(fields[0]) == "Z" && string(fields[1]) == supervisor {
			zombies++
		}
	}
	return zombies
}

func writeJSON(value any) error {
	return json.NewEncoder(os.Stdout).Encode(value)
}
