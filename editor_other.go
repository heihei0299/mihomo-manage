//go:build !windows

package main

import "strings"

func editorArguments(editor string) ([]string, error) { return strings.Fields(editor), nil }
