package models

import (
	"bufio"
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"tomoru/internal/resource"
)

// Device is a GPU llama-server runs models on, and its memory, in bytes: the GPU's own on Windows
// (Vulkan), the memory it may take of a Mac's, which it shares (Metal).
type Device struct {
	Name   string
	Memory uint64
}

// deviceLine is a device as `llama-server --list-devices` lists it:
// "  Vulkan0: NVIDIA GeForce RTX 5060 (7895 MiB, 7127 MiB free)".
var deviceLine = regexp.MustCompile(`^\s+(\S+): (.+) \((\d+) MiB, \d+ MiB free\)$`)

// Devices are the GPUs the llama-server at bin can run models on; none where it has only the CPU.
func Devices(ctx context.Context, bin string) ([]Device, error) {
	cmd := exec.CommandContext(ctx, bin, "--list-devices")
	resource.Hide(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	return parseDevices(out), nil
}

func parseDevices(out []byte) []Device {
	var ds []Device
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		m := deviceLine.FindStringSubmatch(sc.Text())
		if m == nil || strings.HasPrefix(m[1], "BLAS") || strings.HasPrefix(m[1], "CPU") {
			continue // the CPU's libraries are no GPU
		}
		if mib, err := strconv.ParseUint(m[3], 10, 64); err == nil && mib > 0 {
			ds = append(ds, Device{Name: m[2], Memory: mib << 20})
		}
	}
	return ds
}
