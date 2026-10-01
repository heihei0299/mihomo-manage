package manager

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

type windowsCoreService struct{ command func(io.Writer) *exec.Cmd }

func (host windowsCoreService) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}
	fs := OSSystem{}
	if err := fs.MkdirAll(filepath.Dir(ServiceLogPath), dirPermPrivate); err != nil {
		return true, 1
	}
	logFile, err := os.OpenFile(ServiceLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return true, 1
	}
	defer logFile.Close()
	if err := fs.Chmod(ServiceLogPath, filePermPrivateRW); err != nil {
		return true, 1
	}
	command := exec.Command(binaryPath, "-d", configDir)
	if host.command != nil {
		command = host.command(logFile)
	}
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	command.Stdout, command.Stderr = logFile, logFile
	if err := command.Start(); err != nil {
		fmt.Fprintln(logFile, err)
		return true, 1
	}
	job, err := attachCoreJob(command.Process)
	if err != nil {
		fmt.Fprintln(logFile, err)
		_ = command.Process.Kill()
		_ = command.Wait()
		return true, 1
	}
	defer windows.CloseHandle(job)
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	status := svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	changes <- status
	for {
		select {
		case err := <-done:
			if err != nil {
				fmt.Fprintln(logFile, "mihomo exited:", err)
			} else {
				fmt.Fprintln(logFile, "mihomo exited without a service stop request")
			}
			return true, 1
		case request, ok := <-requests:
			if !ok {
				_ = command.Process.Kill()
				<-done
				return false, 0
			}
			switch request.Cmd {
			case svc.Interrogate:
				changes <- status
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				_ = command.Process.Kill()
				<-done
				return false, 0
			}
		}
	}
}

func attachCoreJob(process *os.Process) (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(process.Pid))
	if err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	defer windows.CloseHandle(handle)
	if err := windows.AssignProcessToJobObject(job, handle); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

// RunWindowsService hosts mihomo for the Windows Service Control Manager.
func RunWindowsService() error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !isService {
		return fmt.Errorf("service-run must be started by the Windows service manager")
	}
	return svc.Run(ServiceName, windowsCoreService{})
}
