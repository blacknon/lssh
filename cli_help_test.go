package main

import (
	"strings"
	"testing"

	app_lsdiff "github.com/blacknon/lssh/internal/app/lsdiff"
	app_lspipe "github.com/blacknon/lssh/internal/app/lspipe"
	app_lssh "github.com/blacknon/lssh/internal/app/lssh"
	app_lsshfs "github.com/blacknon/lssh/internal/app/lsshfs"
	app_lsmux "github.com/blacknon/lssh/internal/app/lsmux"
	app_lsshell "github.com/blacknon/lssh/internal/app/lsshell"
)

func TestCLIAppsExposeExpectedVersionStrings(t *testing.T) {
	cases := []struct {
		name    string
		version string
	}{
		{name: "lssh", version: app_lssh.Lssh().Version},
		{name: "lsdiff", version: app_lsdiff.Lsdiff().Version},
		{name: "lsshfs", version: app_lsshfs.Lsshfs().Version},
		{name: "lspipe", version: app_lspipe.Lspipe().Version},
		{name: "lsmux", version: app_lsmux.Lsmux().Version},
		{name: "lsshell", version: app_lsshell.Lsshell().Version},
	}

	for _, tt := range cases {
		if !strings.Contains(tt.version, "lssh-suite 0.9.1") {
			t.Fatalf("%s version = %q", tt.name, tt.version)
		}
	}
}

func TestCLIAppsKeepLoadBearingHelpMetadata(t *testing.T) {
	check := func(t *testing.T, name, usage, version string, flags []string, needles []string, versionNeedle string) {
		t.Helper()
		if usage == "" {
			t.Fatalf("%s usage is empty", name)
		}
		if !strings.Contains(version, versionNeedle) {
			t.Fatalf("%s version = %q, want %q", name, version, versionNeedle)
		}
		allFlags := strings.Join(flags, "\n")
		for _, needle := range needles {
			if !strings.Contains(allFlags, needle) {
				t.Fatalf("%s flags missing %q", name, needle)
			}
		}
	}

	{
		app := app_lssh.Lssh()
		flags := make([]string, 0, len(app.Flags))
		for _, flag := range app.Flags {
			flags = append(flags, flag.String())
		}
		check(t, app.Name, app.Usage, app.Version, flags, []string{"generate-lssh-conf", "enable-control-master", "allow-layout-change"}, "(stable/core)")
	}
	{
		app := app_lsdiff.Lsdiff()
		flags := make([]string, 0, len(app.Flags))
		for _, flag := range app.Flags {
			flags = append(flags, flag.String())
		}
		check(t, app.Name, app.Usage, app.Version, flags, []string{"generate-lssh-conf", "enable-control-master"}, "(beta/sysadmin)")
	}
	{
		app := app_lsshfs.Lsshfs()
		flags := make([]string, 0, len(app.Flags))
		for _, flag := range app.Flags {
			flags = append(flags, flag.String())
		}
		check(t, app.Name, app.Usage, app.Version, flags, []string{"list-mounts", "foreground", "debug", "enable-control-master"}, "(beta/transfer)")
	}
	{
		app := app_lspipe.Lspipe()
		flags := make([]string, 0, len(app.Flags))
		for _, flag := range app.Flags {
			flags = append(flags, flag.String())
		}
		check(t, app.Name, app.Usage, app.Version, flags, []string{"fifo-name", "mkfifo", "raw"}, "(alpha/sysadmin)")
	}
	{
		app := app_lsmux.Lsmux()
		flags := make([]string, 0, len(app.Flags))
		for _, flag := range app.Flags {
			flags = append(flags, flag.String())
		}
		check(t, app.Name, app.Usage, app.Version, flags, []string{"generate-lssh-conf", "allow-layout-change", "enable-control-master"}, "(beta/sysadmin)")
	}
	{
		app := app_lsshell.Lsshell()
		flags := make([]string, 0, len(app.Flags))
		for _, flag := range app.Flags {
			flags = append(flags, flag.String())
		}
		check(t, app.Name, app.Usage, app.Version, flags, []string{"generate-lssh-conf", "enable-control-master", "run specified command at terminal"}, "(beta/sysadmin)")
	}
}
