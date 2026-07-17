package platform

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func installExecutable(source string) (string, error) {
	target, err := installedExecutablePath()
	if err != nil {
		return "", err
	}
	binDir := filepath.Dir(target)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", fmt.Errorf("create application directory: %w", err)
	}
	if samePath(source, target) {
		return target, nil
	}
	input, err := os.Open(source)
	if err != nil {
		return "", fmt.Errorf("open executable: %w", err)
	}
	defer input.Close()
	temp := target + ".tmp"
	output, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return "", fmt.Errorf("create installed executable: %w", err)
	}
	if _, err := io.Copy(output, input); err != nil {
		output.Close()
		return "", fmt.Errorf("copy executable: %w", err)
	}
	if err := output.Close(); err != nil {
		return "", fmt.Errorf("close installed executable: %w", err)
	}
	if err := os.Chmod(temp, 0o755); err != nil {
		return "", fmt.Errorf("make installed executable runnable: %w", err)
	}
	if err := os.Rename(temp, target); err != nil {
		return "", fmt.Errorf("replace installed executable: %w", err)
	}
	return target, nil
}

func InstallCLI(source string) (string, bool, error) {
	target, err := installExecutable(source)
	if err != nil {
		return "", false, err
	}
	added, err := addToUserPath(filepath.Dir(target))
	if err != nil {
		return "", false, err
	}
	return target, added, nil
}

func InstalledExecutablePath() (string, error) {
	return installedExecutablePath()
}

func samePath(a, b string) bool {
	absoluteA, errA := filepath.Abs(a)
	absoluteB, errB := filepath.Abs(b)
	return errA == nil && errB == nil && absoluteA == absoluteB
}
