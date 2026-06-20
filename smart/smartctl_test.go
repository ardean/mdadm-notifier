package smart

import (
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
		_ = CheckDevice("/dev/sdb")
	})
	if len(args) != 2 || args[0] != "-x" || args[1] != "/dev/sdb" {
		t.Fatalf("expected [-x /dev/sdb], got %v", args)
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

type fakeCommandRunner struct{}

func (fakeCommandRunner) CombinedOutput() ([]byte, error) {
	return []byte("SMART overall-health self-assessment test result: PASSED\n"), nil
}
