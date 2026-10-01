package main

import "golang.org/x/sys/windows"

func editorArguments(editor string) ([]string, error) {
	if editor == "" {
		return nil, nil
	}
	return windows.DecomposeCommandLine(editor)
}
