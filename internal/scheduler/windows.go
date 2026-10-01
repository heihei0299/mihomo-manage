package scheduler

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const windowsScheduleName = "mihomo-manager-subscription-update"

const windowsTaskScript = `$ErrorActionPreference='Stop'; [Console]::OutputEncoding=New-Object System.Text.UTF8Encoding; function Test-TaskMissing($e) { while ($null -ne $e) { if ($e.HResult -eq -2147024894) { return $true }; $e=$e.InnerException }; return $false }; $s=New-Object -ComObject Schedule.Service; $s.Connect(); `

type windowsPlatform struct {
	fs      FileSystem
	cmd     CommandRunner
	xmlPath string
}

func NewWindows(fs FileSystem, cmd CommandRunner, xmlPath string) Platform {
	return &windowsPlatform{fs: fs, cmd: cmd, xmlPath: xmlPath}
}

func escapeTaskXML(value string) string {
	var out bytes.Buffer
	_ = xml.EscapeText(&out, []byte(value))
	return out.String()
}

func (s *windowsPlatform) Set(ctx context.Context, interval time.Duration, commandPath string) error {
	if interval < time.Hour || interval > 31*24*time.Hour || interval%time.Second != 0 {
		return fmt.Errorf("Windows schedule interval must be whole seconds between 1h and 744h, got %v", interval)
	}
	task := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <Triggers><TimeTrigger><Repetition><Interval>PT%dS</Interval></Repetition><StartBoundary>%s</StartBoundary><Enabled>true</Enabled></TimeTrigger></Triggers>
  <Principals><Principal id="System"><UserId>S-1-5-18</UserId><LogonType>ServiceAccount</LogonType><RunLevel>HighestAvailable</RunLevel></Principal></Principals>
  <Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries><StopIfGoingOnBatteries>false</StopIfGoingOnBatteries><StartWhenAvailable>true</StartWhenAvailable><Enabled>true</Enabled><ExecutionTimeLimit>PT0S</ExecutionTimeLimit></Settings>
  <Actions Context="System"><Exec><Command>%s</Command><Arguments>subscription update --quiet</Arguments></Exec></Actions>
</Task>
`, int64(interval/time.Second), time.Now().Add(time.Second).Format(time.RFC3339), escapeTaskXML(commandPath))
	if err := s.fs.WriteFile(s.xmlPath, []byte(task), 0600); err != nil {
		return fmt.Errorf("writing Windows scheduled task: %w", err)
	}
	if _, err := s.cmd.RunCommand(ctx, "schtasks.exe", "/Create", "/TN", windowsScheduleName, "/XML", s.xmlPath, "/F"); err != nil {
		return fmt.Errorf("registering Windows scheduled task: %w", err)
	}
	return nil
}

// Query the registered task rather than the XML staging file. COM's numeric
// not-found code is stable across Windows display languages; other errors fail.
func (s *windowsPlatform) taskXML(ctx context.Context) (string, bool, error) {
	script := windowsTaskScript + `try { $task=$s.GetFolder('\').GetTask('` + windowsScheduleName + `'); $task.Xml } catch { if (Test-TaskMissing $_.Exception) { 'MISSING' } else { throw } }`
	out, err := s.cmd.RunCommand(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	if err != nil {
		return "", false, fmt.Errorf("querying Windows scheduled task: %w", err)
	}
	if strings.TrimSpace(out) == "MISSING" {
		return "", false, nil
	}
	if strings.TrimSpace(out) == "" {
		return "", false, fmt.Errorf("Windows scheduled task query returned no definition")
	}
	return out, true, nil
}

func (s *windowsPlatform) Stop(ctx context.Context) error {
	_, exists, err := s.taskXML(ctx)
	if err != nil {
		return err
	}
	if exists {
		script := windowsTaskScript + `try { $folder=$s.GetFolder('\'); $folder.GetTask('` + windowsScheduleName + `').Stop(0); $folder.DeleteTask('` + windowsScheduleName + `',0) } catch { if (-not (Test-TaskMissing $_.Exception)) { throw } }`
		if _, err := s.cmd.RunCommand(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script); err != nil {
			return fmt.Errorf("deleting Windows scheduled task: %w", err)
		}
	}
	return s.fs.RemoveAll(s.xmlPath)
}

func parseTaskInterval(raw string) (time.Duration, error) {
	if !strings.HasPrefix(raw, "P") {
		return 0, fmt.Errorf("invalid Windows task interval %q", raw)
	}
	inTime := false
	var digits strings.Builder
	var total time.Duration
	for _, char := range strings.TrimPrefix(raw, "P") {
		if char == 'T' && digits.Len() == 0 && !inTime {
			inTime = true
			continue
		}
		if char >= '0' && char <= '9' {
			digits.WriteRune(char)
			continue
		}
		value, err := strconv.ParseInt(digits.String(), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid Windows task interval %q", raw)
		}
		digits.Reset()
		var unit time.Duration
		switch {
		case char == 'D' && !inTime:
			unit = 24 * time.Hour
		case char == 'H' && inTime:
			unit = time.Hour
		case char == 'M' && inTime:
			unit = time.Minute
		case char == 'S' && inTime:
			unit = time.Second
		default:
			return 0, fmt.Errorf("invalid Windows task interval %q", raw)
		}
		if value > int64((31*24*time.Hour-total)/unit) {
			return 0, fmt.Errorf("Windows task interval exceeds 31 days")
		}
		total += time.Duration(value) * unit
	}
	if digits.Len() != 0 || total <= 0 {
		return 0, fmt.Errorf("invalid Windows task interval %q", raw)
	}
	return total, nil
}

func (s *windowsPlatform) Status(ctx context.Context) (time.Duration, bool, error) {
	definition, exists, err := s.taskXML(ctx)
	if err != nil || !exists {
		return 0, false, err
	}
	var task struct {
		Enabled  *bool `xml:"Settings>Enabled"`
		Triggers []struct {
			Enabled  *bool  `xml:"Enabled"`
			Interval string `xml:"Repetition>Interval"`
		} `xml:"Triggers>TimeTrigger"`
	}
	decoder := xml.NewDecoder(strings.NewReader(strings.TrimPrefix(definition, "\uFEFF")))
	// COM task XML declares UTF-16, but PowerShell has already written the
	// Unicode string to stdout as UTF-8 using windowsTaskScript's encoding.
	decoder.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		if strings.EqualFold(charset, "utf-16") || strings.EqualFold(charset, "utf-8") {
			return input, nil
		}
		return nil, fmt.Errorf("unsupported Windows task encoding %q", charset)
	}
	if err := decoder.Decode(&task); err != nil {
		return 0, false, fmt.Errorf("decoding Windows scheduled task: %w", err)
	}
	if task.Enabled != nil && !*task.Enabled {
		return 0, false, nil
	}
	for _, trigger := range task.Triggers {
		if trigger.Enabled != nil && !*trigger.Enabled {
			continue
		}
		interval, err := parseTaskInterval(strings.TrimSpace(trigger.Interval))
		return interval, err == nil, err
	}
	if len(task.Triggers) == 0 {
		return 0, false, fmt.Errorf("Windows scheduled task has no time trigger")
	}
	return 0, false, nil
}
