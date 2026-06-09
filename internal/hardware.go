package internal

import (
	"os"
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
	// Probe for an actual device node. A mere `nvidia-smi` on PATH does not mean
	// a usable GPU is present (e.g. driver tools installed on a CPU-only host),
	// and falsely enabling GPU layers leads to a failed/fallback model load.
	_, err := os.Stat("/dev/nvidia0")
	return err == nil
}
