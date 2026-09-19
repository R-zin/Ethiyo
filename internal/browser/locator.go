package browser

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

var (
	// ErrBrowserNotFound is returned when no Chromium-compatible browser could be located.
	ErrBrowserNotFound = errors.New("no supported Chromium-based browser found; set BROWSER_PATH environment variable")
)

// FindBrowserExecutable returns the path to a Chrome, Chromium, Brave, or Edge executable.
// If customPath is provided and exists, it will be returned immediately.
func FindBrowserExecutable(customPath string) (string, error) {
	if customPath != "" {
		if fileExists(customPath) {
			return customPath, nil
		}
		// Also check if customPath is available in PATH
		if p, err := exec.LookPath(customPath); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("configured BROWSER_PATH does not exist: %s", customPath)
	}

	// 1. Check common binary names in PATH
	binaries := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"brave-browser",
		"brave",
		"chrome",
		"msedge",
	}

	for _, bin := range binaries {
		if path, err := exec.LookPath(bin); err == nil {
			return path, nil
		}
	}

	// 2. Check standard OS-specific installation paths
	osPaths := getCandidatePathsForOS()
	for _, p := range osPaths {
		if fileExists(p) {
			return p, nil
		}
	}

	return "", ErrBrowserNotFound
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func getCandidatePathsForOS() []string {
	switch runtime.GOOS {
	case "windows":
		programFiles := os.Getenv("ProgramFiles")
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		localAppData := os.Getenv("LOCALAPPDATA")

		var paths []string
		roots := []string{programFiles, programFilesX86, localAppData}
		subPaths := []string{
			filepath.Join("Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join("BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			filepath.Join("Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join("Chromium", "Application", "chrome.exe"),
		}

		for _, root := range roots {
			if root == "" {
				continue
			}
			for _, sub := range subPaths {
				paths = append(paths, filepath.Join(root, sub))
			}
		}
		return paths

	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			filepath.Join(os.Getenv("HOME"), "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
			filepath.Join(os.Getenv("HOME"), "Applications/Brave Browser.app/Contents/MacOS/Brave Browser"),
		}

	case "linux":
		return []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/usr/bin/brave-browser",
			"/snap/bin/chromium",
			"/snap/bin/brave",
			"/usr/bin/microsoft-edge",
			"/usr/bin/microsoft-edge-stable",
		}

	default:
		return nil
	}
}
