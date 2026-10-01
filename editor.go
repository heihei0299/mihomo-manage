package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func editorCommand(editor, path string) (*exec.Cmd, error) {
	parts, err := editorArguments(editor)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("editor is empty")
	}
	args := append(append([]string{}, parts[1:]...), path)
	return exec.Command(parts[0], args...), nil
}

func configuredEditorCommand(path string) (*exec.Cmd, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
		if runtime.GOOS == "windows" {
			editor = "notepad.exe"
		}
	}
	return editorCommand(editor, path)
}
