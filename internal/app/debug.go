package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
)

func PrintLogTail(out io.Writer, path string, lineCount int) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("service log not found at %s; reinstall the service and switch the KVM once", path)
		}
		return fmt.Errorf("open service log: %w", err)
	}
	defer file.Close()

	return printLastLines(out, file, lineCount)
}

func printLastLines(out io.Writer, input io.Reader, lineCount int) error {
	if lineCount < 1 {
		return fmt.Errorf("line count must be positive")
	}

	lines := make([]string, lineCount)
	total := 0
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines[total%lineCount] = scanner.Text()
		total++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read service log: %w", err)
	}
	if total == 0 {
		_, err := fmt.Fprintln(out, "The service log is empty.")
		return err
	}

	shown := total
	if shown > lineCount {
		shown = lineCount
	}
	start := total - shown
	for i := start; i < total; i++ {
		if _, err := fmt.Fprintln(out, lines[i%lineCount]); err != nil {
			return err
		}
	}
	return nil
}
