package internal

import (
	"os"
	"os/exec"
	"runtime"
)

type Device string

const (
	DeviceMPS  Device = "mps"
	DeviceCUDA Device = "cuda"
	DeviceCPU  Device = "cpu"
)

func DetectHardware() Device {
	if isMPS() {
		return DeviceMPS
	}
	if isCUDA() {
		return DeviceCUDA
	}
	return DeviceCPU
}

func isMPS() bool {
	return runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"
}

func isCUDA() bool {
	if _, err := os.Stat("/dev/nvidia0"); err == nil {
		return true
	}
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		return true
	}
	return false
}
