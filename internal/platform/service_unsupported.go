//go:build !darwin && !windows

package platform

import "fmt"

func installedExecutablePath() (string, error) {
	return "", fmt.Errorf("this operating system is not supported")
}

func addToUserPath(string) (bool, error) {
	return false, fmt.Errorf("this operating system is not supported")
}

func InstallService(string, string) (string, error) {
	return "", fmt.Errorf("this operating system is not supported")
}

func UninstallService() error {
	return fmt.Errorf("this operating system is not supported")
}

func ServiceStatus() (string, error) {
	return "", fmt.Errorf("this operating system is not supported")
}

func ServiceLogPath(string) (string, error) {
	return "", fmt.Errorf("this operating system is not supported")
}

func ApplyUpdate([]byte, string) (string, error) {
	return "", fmt.Errorf("this operating system is not supported")
}
