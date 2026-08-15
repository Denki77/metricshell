package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const fixturePath = "/process-group-fixture"

type observation struct {
	Role string `json:"role"`
	PID  int    `json:"pid"`
	PPID int    `json:"ppid"`
	PGID int    `json:"pgid"`
}

func main() {
	role := os.Getenv("FIXTURE_ROLE")
	if role == "" {
		role = "root"
	}
	processGroupID, err := syscall.Getpgid(0)
	must(err)
	must(json.NewEncoder(os.Stdout).Encode(observation{
		Role: role, PID: os.Getpid(), PPID: os.Getppid(), PGID: processGroupID,
	}))

	switch role {
	case "root":
		runRoot(processGroupID)
	case "child":
		runChild(processGroupID)
	case "grandchild":
		runLeaf(role, processGroupID)
	case "sibling":
		runSibling(processGroupID)
	default:
		fail("unknown role")
	}
}

func runRoot(processGroupID int) {
	if processGroupID != os.Getpid() {
		fail("root is not process-group leader")
	}
	watchSignal("root")

	child := command("child", processGroupID)
	must(child.Start())
	sibling := command("sibling", processGroupID)
	sibling.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	must(sibling.Start())

	waitFor("/ready-child", "/ready-grandchild", "/ready-sibling")
	must(syscall.Kill(-processGroupID, syscall.SIGUSR1))
	waitFor("/signal-root", "/signal-child", "/signal-grandchild")
	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat("/signal-sibling"); err == nil {
		fail("group signal reached unrelated process group")
	}
	must(os.WriteFile("/stop", nil, 0o600))
	must(child.Wait())
	must(sibling.Wait())
	assertNoZombies()
}

func runChild(processGroupID int) {
	assertOwnedGroup(processGroupID)
	watchSignal("child")
	grandchild := command("grandchild", processGroupID)
	must(grandchild.Start())
	waitFor("/ready-grandchild")
	must(os.WriteFile("/ready-child", nil, 0o600))
	waitFor("/stop")
	must(grandchild.Wait())
}

func runLeaf(role string, processGroupID int) {
	assertOwnedGroup(processGroupID)
	watchSignal(role)
	must(os.WriteFile("/ready-"+role, nil, 0o600))
	waitFor("/stop")
}

func runSibling(processGroupID int) {
	ownedGroup := environmentInt("OWNED_PGID")
	if processGroupID != os.Getpid() || processGroupID == ownedGroup {
		fail("sibling process group is not isolated")
	}
	watchSignal("sibling")
	must(os.WriteFile("/ready-sibling", nil, 0o600))
	waitFor("/stop")
}

func command(role string, ownedGroup int) *exec.Cmd {
	command := exec.Command(fixturePath)
	command.Env = append(os.Environ(), "FIXTURE_ROLE="+role, "OWNED_PGID="+strconv.Itoa(ownedGroup))
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command
}

func assertOwnedGroup(processGroupID int) {
	if processGroupID != environmentInt("OWNED_PGID") {
		fail("descendant escaped owned process group")
	}
}

func watchSignal(role string) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1)
	go func() {
		<-signals
		must(os.WriteFile("/signal-"+role, nil, 0o600))
	}()
}

func waitFor(paths ...string) {
	deadline := time.Now().Add(3 * time.Second)
	for _, path := range paths {
		for {
			if _, err := os.Stat(path); err == nil {
				break
			}
			if time.Now().After(deadline) {
				fail("timeout waiting for " + path)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func assertNoZombies() {
	entries, err := filepath.Glob("/proc/[0-9]*/stat")
	must(err)
	for _, entry := range entries {
		contents, err := os.ReadFile(entry)
		must(err)
		fields := strings.Fields(string(contents[strings.LastIndexByte(string(contents), ')')+1:]))
		if len(fields) > 0 && fields[0] == "Z" {
			fail("zombie process remains")
		}
	}
}

func environmentInt(name string) int {
	value, err := strconv.Atoi(os.Getenv(name))
	must(err)
	return value
}

func must(err error) {
	if err != nil {
		fail(err.Error())
	}
}

func fail(message string) {
	_, _ = fmt.Fprintln(os.Stderr, message)
	os.Exit(125)
}
