package scheduler

import (
	"context"
	"os"
	"strings"
)

type fakeFileSystem struct {
	written map[string][]byte
	removed []string
}

func (f *fakeFileSystem) ReadFile(path string) ([]byte, error) {
	if f.written == nil {
		return nil, os.ErrNotExist
	}
	data, ok := f.written[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

func (f *fakeFileSystem) WriteFile(path string, data []byte, _ uint32) error {
	if f.written == nil {
		f.written = map[string][]byte{}
	}
	f.written[path] = append([]byte(nil), data...)
	return nil
}

func (f *fakeFileSystem) RemoveAll(path string) error {
	f.removed = append(f.removed, path)
	for existing := range f.written {
		if existing == path || strings.HasPrefix(existing, path+"/") {
			delete(f.written, existing)
		}
	}
	return nil
}

type commandCall struct {
	name string
	args []string
}

type commandResponse struct {
	output string
	err    error
}

type commandRecorder struct {
	output    string
	cmdErr    error
	responses []commandResponse
	captured  []commandCall
}

func (c *commandRecorder) RunCommand(_ context.Context, name string, args ...string) (string, error) {
	c.captured = append(c.captured, commandCall{name: name, args: append([]string(nil), args...)})
	if len(c.responses) > 0 {
		response := c.responses[0]
		c.responses = c.responses[1:]
		return response.output, response.err
	}
	return c.output, c.cmdErr
}
