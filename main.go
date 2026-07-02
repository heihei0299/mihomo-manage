package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"mihomo-manager/internal/manager"
)

var quiet bool

func quietPrintf(format string, a ...interface{}) {
	if !quiet {
		fmt.Printf(format, a...)
	}
}

func quietPrintln(a ...interface{}) {
	if !quiet {
		fmt.Println(a...)
	}
}

func main() {
	sys := &manager.OSSystem{}
	svc := manager.NewOSServiceManager(sys)
	mgr := manager.New(sys, svc)

	var args []string
	showHelp := false
	showVersion := false

	raw := os.Args[1:]
	for i := 0; i < len(raw); i++ {
		a := raw[i]
		switch a {
		case "--quiet", "-q":
			quiet = true
		case "--help", "-h":
			showHelp = true
		case "--version":
			showVersion = true
		case "-v":
			showVersion = true
		case "-s":
			if i+1 >= len(raw) {
				fmt.Fprintln(os.Stderr, "usage: mihomo-manager -s <url>")
				os.Exit(1)
			}
			i++
			args = append(args, "subscription", "set", raw[i])
		case "-u":
			args = append(args, "subscription", "update")
		case "-c":
			args = append(args, "config", "preview")
		case "-t":
			if i+1 < len(raw) && raw[i+1] == "--off" {
				i++
				args = append(args, "subscription", "schedule", "--off")
			} else if i+2 < len(raw) && raw[i+1] == "--interval" {
				i += 2
				args = append(args, "subscription", "schedule", "--interval", raw[i])
			} else {
				args = append(args, "subscription", "schedule")
			}
		default:
			args = append(args, a)
		}
	}

	if showVersion {
		fmt.Println("mihomo-manager v0.1.0")
		return
	}
	if showHelp {
		printUsage()
		return
	}
	if len(args) == 0 {
		if err := startTUI(mgr); err != nil {
			log.Fatal(err)
		}
		return
	}

	switch args[0] {
	case "i":
		args[0] = "install"
	case "ui":
		args[0] = "uninstall"
	case "ug":
		args[0] = "upgrade"
	case "v":
		args[0] = "versions"
	}

	switch args[0] {
	case "status":
		cliStatus(mgr)
		return
	case "install":
		version := "latest"
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			version = args[1]
		}
		cliInstall(mgr, version)
		return
	case "template":
		cliEditFile(mgr, manager.ConfigTemplatePath, args[1:])
		return
	case "rules":
		cliEditFile(mgr, manager.RoutingRulesPath, args[1:])
		return
	case "config":
		cliConfig(mgr, args[1:])
		return
	case "subscription":
		cliSubscription(mgr, args[1:])
		return
	case "start":
		cliSimpleOp(mgr.Start, "started")
		return
	case "stop":
		cliSimpleOp(mgr.Stop, "stopped")
		return
	case "restart":
		cliSimpleOp(mgr.Restart, "restarted")
		return
	case "reload":
		cliSimpleOp(mgr.Reload, "reloaded")
		return
	case "uninstall":
		keepBackup := false
		for _, a := range args[1:] {
			if a == "--keep-backup" {
				keepBackup = true
			}
		}
		cliUninstall(mgr, keepBackup)
		return
	case "upgrade":
		version := "latest"
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			version = args[1]
		}
		cliUpgrade(mgr, version)
		return
	case "logs":
		cliLogs(args[1:])
		return
	case "versions":
		cliVersions(mgr)
		return
	}
}

func cliStatus(mgr manager.Manager) {
	status, err := mgr.Status(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if !status.Installed {
		quietPrintln("mihomo: not installed")
		os.Exit(2)
	}

	switch status.InstanceState {
	case manager.Running:
		quietPrintf("mihomo: running  (version: %s)\n", status.Version)
		os.Exit(0)
	case manager.Stopped:
		quietPrintf("mihomo: stopped  (version: %s)\n", status.Version)
		os.Exit(1)
	case manager.Upgrading:
		quietPrintln("mihomo: upgrading")
		os.Exit(1)
	default:
		quietPrintf("mihomo: %s\n", strings.ToLower(status.InstanceState.String()))
		os.Exit(1)
	}
}

func cliInstall(mgr manager.Manager, version string) {
	err := mgr.Install(context.Background(), version, func(e manager.ProgressEvent) {
		prefix := "  "
		if e.Error != nil {
			prefix = "✗ "
		} else if strings.HasSuffix(e.Message, "complete") || strings.HasSuffix(e.Message, "running") {
			prefix = "✓ "
		}
		fmt.Printf("%s[%s] %s\n", prefix, e.Phase, e.Message)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "install failed: %v\n", err)
		os.Exit(1)
	}
	quietPrintln("mihomo installed successfully")
}

func cliConfig(mgr manager.Manager, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mihomo-manager config preview")
		os.Exit(1)
	}
	switch args[0] {
	case "preview":
		preview, err := mgr.PreviewConfig(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(preview)
	default:
		fmt.Fprintf(os.Stderr, "unknown config subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func cliSubscription(mgr manager.Manager, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mihomo-manager subscription <set|update>")
		os.Exit(1)
	}
	switch args[0] {
	case "set":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: mihomo-manager subscription set <url-or-data>")
			os.Exit(1)
		}
		data := strings.Join(args[1:], " ")
		if err := mgr.SetSubscriptionSource(context.Background(), data); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		quietPrintln("subscription saved")
	case "update":
		if err := mgr.UpdateConfig(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		quietPrintln("config updated")
	case "schedule":
		cliSchedule(mgr, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subscription subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func cliSchedule(mgr manager.Manager, args []string) {
	if len(args) == 0 {
		interval, active, _ := mgr.ScheduleStatus(context.Background())
		if active {
			fmt.Printf("schedule: every %v\n", interval)
		} else {
			quietPrintln("schedule: off")
		}
		return
	}
	switch args[0] {
	case "--interval":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: mihomo-manager subscription schedule --interval <duration>")
			os.Exit(1)
		}
		d, err := time.ParseDuration(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid duration: %v\n", err)
			os.Exit(1)
		}
		if err := mgr.SetSchedule(context.Background(), d); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("schedule set to every %v\n", d)
	case "--off":
		if err := mgr.StopSchedule(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		quietPrintln("schedule stopped")
	default:
		fmt.Fprintf(os.Stderr, "unknown schedule option: %s\n", args[0])
		os.Exit(1)
	}
}

func cliUninstall(mgr manager.Manager, keepBackup bool) {
	err := mgr.Uninstall(context.Background(), keepBackup, func(e manager.ProgressEvent) {
		prefix := "  "
		if e.Error != nil {
			prefix = "✗ "
		}
		fmt.Printf("%s[%s] %s\n", prefix, e.Phase, e.Message)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "uninstall failed: %v\n", err)
		os.Exit(1)
	}
	quietPrintln("mihomo uninstalled")
}

func cliUpgrade(mgr manager.Manager, version string) {
	err := mgr.Upgrade(context.Background(), version, func(e manager.ProgressEvent) {
		prefix := "  "
		if e.Error != nil {
			prefix = "✗ "
		}
		fmt.Printf("%s[%s] %s\n", prefix, e.Phase, e.Message)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "upgrade failed: %v\n", err)
		os.Exit(1)
	}
	quietPrintln("upgrade complete")
}

func cliLogs(args []string) {
	follow := false
	tail := 50
	for _, a := range args {
		if a == "--follow" || a == "-f" {
			follow = true
		}
		if strings.HasPrefix(a, "--tail=") {
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--tail="))
			if err == nil {
				tail = n
			}
		}
	}
	journalArgs := []string{"-u", "mihomo", "-n", fmt.Sprintf("%d", tail)}
	if follow {
		journalArgs = append(journalArgs, "--follow")
	}
	cmd := exec.Command("journalctl", journalArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "logs failed: %v\n", err)
		os.Exit(1)
	}
}

func cliVersions(mgr manager.Manager) {
	versions, err := mgr.ListVersions(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	for _, v := range versions {
		fmt.Println(v.Tag)
	}
}

func cliEditFile(mgr manager.Manager, path string, args []string) {
	if len(args) != 1 || args[0] != "edit" {
		fmt.Fprintf(os.Stderr, "usage: mihomo-manager %s edit\n", path)
		os.Exit(1)
	}
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "editor failed: %v\n", err)
		os.Exit(1)
	}
	if err := mgr.UpdateConfig(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "config update failed: %v\n", err)
		os.Exit(1)
	}
	quietPrintln("config updated")
}

type commandFunc func(context.Context) error

func printUsage() {
	fmt.Println(`Usage of mihomo-manager:
  -c
    	Preview generated config (alias: config preview)
  -h, --help
    	Show this help
  -q, --quiet
    	Suppress non-error output
  -s string
    	Set subscription source (alias: subscription set)
  -t [--interval duration|--off]
    	View/configure auto-refresh (alias: subscription schedule)
  -u
    	Refresh and apply subscription (alias: subscription update)
  -v, --version
    	Show version
  config preview
    	Preview generated config
  i [version]
    	Install mihomo (alias: install, default: latest)
  install [version]
    	Install mihomo (default: latest)
  logs [--tail=N] [--follow]
    	View mihomo logs
  reload
    	Reload config
  restart
    	Restart mihomo
  rules edit
    	Edit routing rules ($EDITOR)
  start
    	Start mihomo
  status
    	Show mihomo status
  stop
    	Stop mihomo
  subscription schedule [--interval duration|--off]
    	View/configure auto-refresh
  subscription set string
    	Set subscription source
  subscription update
    	Refresh and apply subscription
  template edit
    	Edit config template ($EDITOR)
  ug [version]
    	Upgrade mihomo (alias: upgrade, default: latest)
  ui [--keep-backup]
    	Uninstall mihomo (alias: uninstall)
  uninstall [--keep-backup]
    	Remove mihomo
  upgrade [version]
    	Upgrade mihomo (default: latest)
  v
    	List available versions (alias: versions)
  versions
    	List available versions

Run without arguments to start the TUI.`)
}

func cliSimpleOp(op commandFunc, verb string) {
	if err := op(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	quietPrintf("mihomo %s\n", verb)
}
