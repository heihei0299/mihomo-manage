package main

import (
	"reflect"
	"testing"
)

func TestWindowsEditorPreservesQuotedPathAndArguments(t *testing.T) {
	command, err := editorCommand(`"C:\Program Files\Editor\editor.exe" --wait`, `C:\ProgramData\mihomo\etc\override.yaml`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`C:\Program Files\Editor\editor.exe`, "--wait", `C:\ProgramData\mihomo\etc\override.yaml`}
	if !reflect.DeepEqual(command.Args, want) {
		t.Fatalf("args = %v", command.Args)
	}
}
