// Package wsl detects WSL2 and (best-effort) lists Windows host adapters.
package wsl

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/chinmay-sawant/openwire/internal/domain"
)

// IsWSL2 reports whether the process appears to run under WSL2.
func IsWSL2() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	v := strings.ToLower(string(data))
	return strings.Contains(v, "microsoft") || strings.Contains(v, "wsl")
}

// ListWindowsHostAdapters best-effort inventories Windows adapters from inside WSL.
// Returns nil when not on WSL or when the host tooling is unavailable.
func ListWindowsHostAdapters(ctx context.Context) []domain.Adapter {
	if !IsWSL2() {
		return nil
	}
	// Prefer PowerShell; fall back to ipconfig parsing.
	if adapters := fromPowerShell(ctx); len(adapters) > 0 {
		return adapters
	}
	return fromIPConfig(ctx)
}

func fromPowerShell(ctx context.Context) []domain.Adapter {
	ps := powershellPath()
	if ps == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ps, "-NoProfile", "-Command",
		"Get-NetAdapter | Select-Object Name,Status,MacAddress,ifIndex | ConvertTo-Csv -NoTypeInformation")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(string(out), "\n")
	var adapters []domain.Adapter
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || i == 0 {
			continue
		}
		fields := parseCSVLine(line)
		if len(fields) < 3 {
			continue
		}
		name := strings.Trim(fields[0], `"`)
		status := strings.ToLower(strings.Trim(fields[1], `"`))
		mac := strings.Trim(fields[2], `"`)
		idx := 0
		if len(fields) >= 4 {
			idx = atoi(strings.Trim(fields[3], `"`))
		}
		adapters = append(adapters, domain.Adapter{
			Name:     name,
			Index:    idx,
			Hardware: mac,
			Up:       status == "up",
			Source:   domain.AdapterSourceWindowsHost,
		})
	}
	return adapters
}

func fromIPConfig(ctx context.Context) []domain.Adapter {
	bin := ipconfigPath()
	if bin == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "/all")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var adapters []domain.Adapter
	var current *domain.Adapter
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// Adapter section headers end with ":" and are not indented in typical ipconfig output.
		if strings.HasSuffix(line, ":") && !strings.Contains(line, ".") {
			name := strings.TrimSuffix(line, ":")
			if current != nil {
				adapters = append(adapters, *current)
			}
			current = &domain.Adapter{
				Name:   name,
				Source: domain.AdapterSourceWindowsHost,
				Up:     true,
			}
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(line, "Physical Address") || strings.HasPrefix(line, "物理地址") {
			if _, val, ok := strings.Cut(line, ":"); ok {
				current.Hardware = strings.TrimSpace(val)
			}
		}
		if strings.Contains(line, "IPv4") {
			if _, val, ok := strings.Cut(line, ":"); ok {
				ip := strings.TrimSpace(val)
				ip = strings.Split(ip, "(")[0]
				ip = strings.TrimSpace(ip)
				if ip != "" {
					current.IPv4 = append(current.IPv4, ip)
				}
			}
		}
		if strings.HasPrefix(line, "Media State") && strings.Contains(strings.ToLower(line), "disconnect") {
			current.Up = false
		}
	}
	if current != nil {
		adapters = append(adapters, *current)
	}
	return adapters
}

func parseCSVLine(line string) []string {
	var fields []string
	var cur strings.Builder
	inQuotes := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '"':
			inQuotes = !inQuotes
		case c == ',' && !inQuotes:
			fields = append(fields, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	fields = append(fields, cur.String())
	return fields
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
