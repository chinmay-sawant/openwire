package wsl

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

var (
	psOnce   sync.Once
	psPath   string
	ipcfgOnce sync.Once
	ipcfgPath string
)

// powershellPath resolves powershell.exe for WSL2 even when it is not on PATH.
func powershellPath() string {
	psOnce.Do(func() {
		if p, err := exec.LookPath("powershell.exe"); err == nil {
			psPath = p
			return
		}
		// Common WSL mounts for the Windows system drive.
		candidates := []string{
			"/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe",
			"/mnt/c/WINDOWS/System32/WindowsPowerShell/v1.0/powershell.exe",
		}
		if windir := os.Getenv("WINDIR"); windir != "" {
			// Sometimes exposed as /mnt/c/Windows via WINDIR=C:\Windows
			candidates = append(candidates,
				filepath.Join("/mnt/c", "Windows", "System32", "WindowsPowerShell", "v1.0", "powershell.exe"),
			)
		}
		for _, c := range candidates {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				psPath = c
				return
			}
		}
	})
	return psPath
}

// ipconfigPath resolves ipconfig.exe similarly.
func ipconfigPath() string {
	ipcfgOnce.Do(func() {
		if p, err := exec.LookPath("ipconfig.exe"); err == nil {
			ipcfgPath = p
			return
		}
		for _, c := range []string{
			"/mnt/c/Windows/System32/ipconfig.exe",
			"/mnt/c/WINDOWS/System32/ipconfig.exe",
		} {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				ipcfgPath = c
				return
			}
		}
	})
	return ipcfgPath
}
