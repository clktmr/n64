// test-runner runs a command as a subprocess and scans the output for passed or
// failed tests. If one such message is found, the subprocess will be sent a
// SIGTERM after a short delay. The exit code will be 0 if all tests passed,
// otherwise 1.
package main

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func main() {
	log.Default().SetFlags(0)
	cmd := exec.Command(os.Args[1], os.Args[2:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal("open stdout:", err)
	}

	err = cmd.Start()
	if err != nil {
		log.Fatal("start command:", err)
	}

	scanner := bufio.NewScanner(stdout)
	exiting := false
	code := 0
	for scanner.Scan() {
		log.Println(scanner.Text())
		if exiting {
			continue
		}
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "fatal error:"), strings.HasPrefix(line, "panic:"):
			fallthrough
		case line == "FAIL":
			exiting = true
			code = 1
			go exitCmd(cmd)
		case line == "PASS":
			exiting = true
			go exitCmd(cmd)
		}
	}
	cmd.Wait()
	os.Exit(code)
}

func exitCmd(cmd *exec.Cmd) {
	time.Sleep(500 * time.Millisecond)
	syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
}
