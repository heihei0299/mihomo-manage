package manager

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows/svc"
)

func TestWindowsCoreServiceChild(t *testing.T) {
	if os.Getenv("MIHOMO_CORE_TEST_CHILD") != "1" {
		return
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestWindowsCoreServiceHandlesStopAndUnexpectedExit(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, stop := range []bool{true, false} {
		t.Run(map[bool]string{true: "stop", false: "unexpected exit"}[stop], func(t *testing.T) {
			previous := ServiceLogPath
			ServiceLogPath = filepath.Join(t.TempDir(), "logs", "mihomo.log")
			t.Cleanup(func() { ServiceLogPath = previous })
			var child *exec.Cmd
			host := windowsCoreService{command: func(io.Writer) *exec.Cmd {
				child = exec.Command(executable, "-test.run=^TestWindowsCoreServiceChild$")
				child.Env = append(os.Environ(), "MIHOMO_CORE_TEST_CHILD=1")
				return child
			}}
			requests := make(chan svc.ChangeRequest, 1)
			changes := make(chan svc.Status, 4)
			type result struct {
				specific bool
				code     uint32
			}
			done := make(chan result, 1)
			go func() { specific, code := host.Execute(nil, requests, changes); done <- result{specific, code} }()
			timeout := time.NewTimer(10 * time.Second)
			defer timeout.Stop()
			for running := false; !running; {
				select {
				case status := <-changes:
					running = status.State == svc.Running
				case ended := <-done:
					t.Fatalf("host exited before running: %+v", ended)
				case <-timeout.C:
					t.Fatal("host startup timed out")
				}
			}
			if stop {
				requests <- svc.ChangeRequest{Cmd: svc.Stop}
			} else if err := child.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			select {
			case ended := <-done:
				if stop && (ended.specific || ended.code != 0) {
					t.Fatalf("normal stop = %+v", ended)
				}
				if !stop && (!ended.specific || ended.code == 0) {
					t.Fatalf("unexpected exit = %+v", ended)
				}
			case <-timeout.C:
				_ = child.Process.Kill()
				t.Fatal("host shutdown timed out")
			}
		})
	}
}
