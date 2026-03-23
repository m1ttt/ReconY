package tools

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// ExecResult holds the result of running an external tool.
type ExecResult struct {
	ExitCode int
	Duration time.Duration
}

// LineCB is called for each line of stdout/stderr output.
type LineCB func(stream, line string)

// RunTool executes an external command with line-by-line streaming.
// The lineCb function is called for each line of output as it arrives.
func RunTool(ctx context.Context, name string, args []string, lineCb LineCB) (*ExecResult, error) {
	return RunToolWithProxy(ctx, name, args, "", lineCb)
}

// RunToolWithProxy executes a command with optional proxy env var injection.
// It creates a new process group so that all child processes (e.g., headless
// browsers spawned by katana) are killed when the context is cancelled.
func RunToolWithProxy(ctx context.Context, name string, args []string, proxyURL string, lineCb LineCB) (*ExecResult, error) {
	start := time.Now()

	// Use exec.Command (NOT CommandContext) — we handle cancellation ourselves
	// via process group killing, which is more thorough than Go's default
	// behavior of killing only the main PID.
	cmd := exec.Command(name, args...)

	// Create a new process group so we can kill all descendants
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Inject proxy env vars for child process
	if proxyURL != "" {
		cmd.Env = append(os.Environ(),
			"HTTP_PROXY="+proxyURL,
			"HTTPS_PROXY="+proxyURL,
			"ALL_PROXY="+proxyURL,
		)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", name, err)
	}

	pgid := cmd.Process.Pid // with Setpgid, pgid == pid of the leader

	// Watchdog: kill the entire process group on context cancellation.
	// SIGTERM first, then SIGKILL after 3 seconds if still alive.
	// Also close pipes to unblock scanner goroutines.
	watchdogDone := make(chan struct{})
	go func() {
		defer close(watchdogDone)
		<-ctx.Done()
		// SIGTERM the process group (negative PID = group)
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		// Close pipes to unblock readers immediately
		stdout.Close()
		stderr.Close()
		// Escalate to SIGKILL after 3 seconds
		time.AfterFunc(3*time.Second, func() {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		})
	}()

	// Read stdout and stderr concurrently
	readersDone := make(chan struct{}, 2)

	readPipe := func(pipe io.Reader, stream string) {
		defer func() { readersDone <- struct{}{} }()
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer
		for scanner.Scan() {
			if lineCb != nil {
				lineCb(stream, scanner.Text())
			}
		}
	}

	go readPipe(stdout, "stdout")
	go readPipe(stderr, "stderr")

	// Wait for both readers to finish (they'll unblock when pipes close)
	<-readersDone
	<-readersDone

	// Wait for the process to exit, with a safety timeout
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	select {
	case err = <-waitCh:
		// Process exited normally
	case <-time.After(5 * time.Second):
		// cmd.Wait() stuck — force kill and wait
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		err = <-waitCh
	}

	result := &ExecResult{
		Duration: time.Since(start),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			// Context cancellation → report as killed, not error
			if ctx.Err() != nil {
				return result, nil
			}
		} else if ctx.Err() != nil {
			// Context cancelled — not an error, just timeout/cancel
			return result, nil
		} else {
			return result, fmt.Errorf("waiting for %s: %w", name, err)
		}
	}

	return result, nil
}

// CheckBinary verifies that a binary exists in PATH.
func CheckBinary(name string) error {
	_, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s not found in PATH", name)
	}
	return nil
}
