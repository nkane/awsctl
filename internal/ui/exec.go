package ui

import (
	"fmt"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	awsx "github.com/nkane/awsctl/internal/aws"
)

// execErrMsg reports that an exec could not be started (e.g. missing binary).
type execErrMsg struct{ err string }

// execDoneMsg is delivered after an interactive exec session ends.
type execDoneMsg struct{ err error }

// ecsExecArgs builds the `aws ecs execute-command` argument list for an
// interactive shell into a container. Region/profile mirror the active config
// so exec targets the same account/region as the TUI.
func ecsExecArgs(cfg *awsx.Config, cluster, task, container string) []string {
	args := []string{
		"ecs", "execute-command",
		"--cluster", cluster,
		"--task", task,
		"--container", container,
		"--interactive",
		"--command", "/bin/sh",
	}
	if cfg != nil {
		if cfg.Region != "" {
			args = append(args, "--region", cfg.Region)
		}
		if cfg.Profile != "" {
			args = append(args, "--profile", cfg.Profile)
		}
	}
	return args
}

// ecsExecCmd preflights and returns a command that runs the interactive exec
// session via tea.ExecProcess (suspending the TUI), or — if the AWS CLI is not
// installed — a command that emits an execErrMsg.
func (a App) ecsExecCmd(cluster, task, container string) tea.Cmd {
	if _, err := exec.LookPath("aws"); err != nil {
		return func() tea.Msg {
			return execErrMsg{err: "ECS Exec needs the AWS CLI and session-manager-plugin on PATH (aws not found)"}
		}
	}
	c := exec.Command("aws", ecsExecArgs(a.cfg, cluster, task, container)...) //nolint:gosec // fixed argv, no shell
	return tea.ExecProcess(c, func(err error) tea.Msg { return execDoneMsg{err: err} })
}

// execErrText formats an exec failure for the status bar.
func execErrText(err error) string {
	return fmt.Sprintf("exec failed: %v", err)
}
