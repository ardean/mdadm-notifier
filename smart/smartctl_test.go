package smart

import (
	"fmt"
	"testing"
)

func TestSmartctlDeviceInfoUsesExtendedOutput(t *testing.T) {
	args := captureSmartctlArgs(t, func() {
		_, _ = readDeviceOutput("/dev/sdb")
	})
	if len(args) != 2 || args[0] != "-x" || args[1] != "/dev/sdb" {
		t.Fatalf("expected [-x /dev/sdb], got %v", args)
	}
}

func TestSmartctlSelfTestLogUsesExtendedOutput(t *testing.T) {
	args := captureSmartctlArgs(t, func() {
		_, _ = readSelfTestOutput("/dev/sdb")
	})
	if len(args) != 3 || args[0] != "-l" || args[1] != "xselftest,selftest" || args[2] != "/dev/sdb" {
		t.Fatalf("expected [-l xselftest,selftest /dev/sdb], got %v", args)
	}
}

func TestCheckDeviceUsesExtendedOutput(t *testing.T) {
	args := captureSmartctlArgs(t, func() {
		_ = CheckDevice("/dev/sdb", CheckOptions{})
	})
	if len(args) != 2 || args[0] != "-x" || args[1] != "/dev/sdb" {
		t.Fatalf("expected [-x /dev/sdb], got %v", args)
	}
}

func TestInterpretSmartctlHealthExitCode(t *testing.T) {
	err := interpretSmartctlError("smartctl -x /dev/sdd", []byte(seagateMarginalOutput), exitStatusError{code: 64})
	if err != nil {
		t.Fatalf("expected health exit code to be ignored, got %v", err)
	}
}

func TestInterpretSmartctlOpenFailure(t *testing.T) {
	output := "Smartctl open device: /dev/sdd failed: No such device\n"
	err := interpretSmartctlError("smartctl -x /dev/sdd", []byte(output), exitStatusError{code: 2})
	if err == nil {
		t.Fatal("expected open failure error")
	}
	if got := err.Error(); got != "smartctl -x /dev/sdd: No such device" {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestCheckDeviceHealthExitCodeStillEvaluates(t *testing.T) {
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })

	execCommand = func(name string, args ...string) commandRunner {
		return fakeCommandRunner{output: seagateMarginalOutput, exitCode: 64}
	}

	result := CheckDevice("/dev/sdd1", CheckOptions{
		ReallocatedThreshold:   1,
		UncorrectableThreshold: 1,
		PendingThreshold:       1,
		OfflineThreshold:       1,
		ErrorLogThreshold:      1,
	})

	if result.ReadOK != true {
		t.Fatalf("expected successful read despite exit code 64, ReadOK=%v", result.ReadOK)
	}
	if result.Healthy {
		t.Fatalf("expected unhealthy disk from critical signals, got:\n%s", result.Summary)
	}
}

func TestCheckDeviceOpenFailureMessage(t *testing.T) {
	orig := execCommand
	t.Cleanup(func() { execCommand = orig })

	execCommand = func(name string, args ...string) commandRunner {
		return fakeCommandRunner{
			output:   "Smartctl open device: /dev/sdd failed: No such device\n",
			exitCode: 2,
		}
	}

	result := CheckDevice("/dev/sdd1", CheckOptions{})
	if result.ReadOK {
		t.Fatal("expected read failure")
	}
	want := "/dev/sdd1: SMART read failed — No such device"
	if result.Summary != want {
		t.Fatalf("summary = %q, want %q", result.Summary, want)
	}
}

func captureSmartctlArgs(t *testing.T, fn func()) []string {
	t.Helper()

	orig := execCommand
	t.Cleanup(func() { execCommand = orig })

	var captured []string
	execCommand = func(name string, args ...string) commandRunner {
		if name != "smartctl" {
			t.Fatalf("expected smartctl command, got %q", name)
		}
		captured = append([]string{}, args...)
		return fakeCommandRunner{}
	}

	fn()
	return captured
}

type fakeCommandRunner struct {
	output   string
	exitCode int
}

func (f fakeCommandRunner) CombinedOutput() ([]byte, error) {
	out := f.output
	if out == "" {
		out = "SMART overall-health self-assessment test result: PASSED\n"
	}
	if f.exitCode == 0 {
		return []byte(out), nil
	}
	return []byte(out), exitStatusError{code: f.exitCode}
}

type exitStatusError struct {
	code int
}

func (e exitStatusError) Error() string {
	return fmt.Sprintf("exit status %d", e.code)
}

func (e exitStatusError) ExitCode() int {
	return e.code
}
