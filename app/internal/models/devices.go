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

type Device struct {
	Name   string
	Memory uint64
}

var deviceLine = regexp.MustCompile(`^\s+(\S+): (.+) \((\d+) MiB, \d+ MiB free\)$`)

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
			continue
		}
		if mib, err := strconv.ParseUint(m[3], 10, 64); err == nil && mib > 0 {
			ds = append(ds, Device{Name: m[2], Memory: mib << 20})
		}
	}
	return ds
}
