package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
"github.com/anomalyco/mihomo-manager/internal/cli"

	"github.com/anomalyco/mihomo-manager/internal/manager"
)

var (
	version   = "dev"
	quietMode bool
)

func main() {
	oss := &manager.OSSystem{}
	svcMgr := manager.NewOSServiceManager(oss, oss)
	ctrl := manager.NewServiceControl(oss, oss, svcMgr)
	lifecycle := manager.NewLifecycleManager(oss, oss, oss, svcMgr)
	cfg := manager.NewConfigManager(oss, oss, manager.NewConfigValidator(), func(ctx context.Context) error {
		return svcMgr.Reload(manager.ServiceName)
	})
	sched := manager.NewScheduleManager(oss, func(ctx context.Context) {
		if err := cfg.UpdateConfig(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "schedule: update failed: %v\n", err)
		}
	})

	var args []string
	showHelp := false
	showVersion := false

	raw := os.Args[1:]
	for i := 0; i < len(raw); i++ {
		a := raw[i]
		switch a {
		case "--quiet", "-q":
			quietMode = true
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
		case "-i":
			args = append(args, "install")
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
		fmt.Printf("mihomo-manager %s\n", version)
		return
	}
	if showHelp {
		printUsage()
		return
	}

	stdout := io.Writer(os.Stdout)
	if quietMode {
		stdout = io.Discard
	}
	h := cli.New(ctrl, lifecycle, cfg, sched, stdout, os.Stderr)

	if len(args) == 0 {
		if err := startTUI(ctrl, lifecycle, cfg); err != nil {
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

	ctx := context.Background()
	var exitCode int

	switch args[0] {
	case "status":
		exitCode = h.Status(ctx)
	case "install":
		ver := "latest"
		autoStart := true
		for _, a := range args[1:] {
			if a == "--no-autostart" {
				autoStart = false
			} else if !strings.HasPrefix(a, "-") {
				ver = a
			}
		}
		exitCode = h.Install(ctx, ver, autoStart)
	case "autostart":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: mihomo-manager autostart on|off")
			os.Exit(1)
		}
		switch args[1] {
		case "on":
			exitCode = h.AutoStart(ctx, true)
		case "off":
			exitCode = h.AutoStart(ctx, false)
		default:
			fmt.Fprintf(os.Stderr, "usage: mihomo-manager autostart on|off\n")
			exitCode = 1
		}
	case "config":
		if len(args) < 2 {
			printUsage()
			exitCode = 1
			break
		}
		switch args[1] {
		case "preview":
			exitCode = h.PreviewConfig(ctx)
		case "template", "rules":
			path := manager.ConfigTemplatePath
			if args[1] == "rules" {
				path = manager.RoutingRulesPath
			}
			cliEditFile(cfg, path, args[2:])
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown config subcommand: %s\n", args[1])
			exitCode = 1
		}
	case "subscription":
		exitCode = handleSubscription(h, ctx, args[1:])
	case "start":
		exitCode = h.Start(ctx)
	case "stop":
		exitCode = h.Stop(ctx)
	case "restart":
		exitCode = h.Restart(ctx)
	case "reload":
		exitCode = h.Reload(ctx)
	case "uninstall":
		keepBackup := false
		for _, a := range args[1:] {
			if a == "--keep-backup" {
				keepBackup = true
			}
		}
		exitCode = h.Uninstall(ctx, keepBackup)
	case "upgrade":
		ver := "latest"
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			ver = args[1]
		}
		exitCode = h.Upgrade(ctx, ver)
	case "logs":
		cliLogs(args[1:])
		return
	case "versions":
		exitCode = h.Versions(ctx)
	default:
		exitCode = 1
		printUsage()
	}

	os.Exit(exitCode)
}

func handleSubscription(h *cli.Handler, ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: mihomo-manager subscription <set|update|schedule>")
		return 1
	}
	switch args[0] {
	case "set":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: mihomo-manager subscription set <url-or-data>")
			return 1
		}
		return h.SetSubscription(ctx, strings.Join(args[1:], " "))
	case "update":
		return h.UpdateConfig(ctx)
	case "schedule":
		return handleSchedule(h, ctx, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subscription subcommand: %s\n", args[0])
		return 1
	}
}

func handleSchedule(h *cli.Handler, ctx context.Context, args []string) int {
	if len(args) == 0 {
		return h.ScheduleStatus(ctx)
	}
	switch args[0] {
	case "--interval":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: mihomo-manager subscription schedule --interval <duration>")
			return 1
		}
		d, err := time.ParseDuration(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid duration: %v\n", err)
			return 1
		}
		return h.SetSchedule(ctx, d)
	case "--off":
		return h.StopSchedule(ctx)
	default:
		fmt.Fprintf(os.Stderr, "unknown schedule option: %s\n", args[0])
		return 1
	}
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

func cliEditFile(cfg manager.ConfigManager, path string, args []string) {
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
	if err := cfg.UpdateConfig(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "config update failed: %v\n", err)
		os.Exit(1)
	}
	if !quietMode {
		fmt.Println("config updated")
	}
}

func printUsage() {
	fmt.Println(`Usage: mihomo-manager [command]

Flags:
  -c              Preview generated config
  -h, --help      Show this help
  -i              Install mihomo (alias: install)
  -q, --quiet     Suppress non-error output
  -s <url>        Set subscription source
  -t [--interval|--off]  View/configure auto-refresh
  -u              Refresh and apply subscription
  -v, --version   Show version

Basic operations:
  status                  Show mihomo status
  start                   Start mihomo
  stop                    Stop mihomo
  restart                 Restart mihomo
  reload                  Reload config
  logs [--tail=N] [--follow]  View mihomo logs

Subscription:
  subscription set <s>        Set subscription source
  subscription update         Refresh and apply subscription
  subscription schedule [--interval|--off]  View/configure auto-refresh

Config:
  config preview              Preview generated config
  config template             Edit config template ($EDITOR)
  config rules                Edit routing rules ($EDITOR)

Lifecycle:
  install/i [ver] [--no-autostart]   Install mihomo (default: latest)
  upgrade/ug [ver]                    Upgrade mihomo (default: latest)
  uninstall/ui [--keep-backup]        Remove mihomo
  autostart on|off                    Toggle auto-start on boot
  versions/v                          List available versions

Run without arguments to start the TUI.`)
}


