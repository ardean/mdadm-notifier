package smart

import (
	"os/exec"
)

type commandRunner interface {
	CombinedOutput() ([]byte, error)
}

var execCommand = func(name string, args ...string) commandRunner {
	return exec.Command(name, args...)
}

func runSmartctl(device string, args ...string) ([]byte, error) {
	cmdArgs := append(append([]string{}, args...), device)
	return execCommand("smartctl", cmdArgs...).CombinedOutput()
}

func readDeviceOutput(device string) ([]byte, error) {
	return runSmartctl(NormalizeDevice(device), "-x")
}

func readSelfTestOutput(device string) ([]byte, error) {
	return runSmartctl(NormalizeDevice(device), "-l", "xselftest,selftest")
}
