package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"time"
)

func streamLogFile(ctx context.Context, path string, output io.Writer, tail int, follow bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	lines := make([]string, 0)
	for {
		line, readErr := reader.ReadString('\n')
		if line != "" && tail > 0 {
			if len(lines) == tail {
				copy(lines, lines[1:])
				lines = lines[:len(lines)-1]
			}
			lines = append(lines, line)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	for _, line := range lines {
		if _, err := io.WriteString(output, line); err != nil {
			return err
		}
	}
	if !follow {
		return nil
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			if _, writeErr := io.WriteString(output, line); writeErr != nil {
				return writeErr
			}
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if err == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
