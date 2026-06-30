package ecs

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	awsx "github.com/nkane/awsctl/internal/aws"
)

// ECS write-action audit labels (recorded by the App's confirm gate, and used
// by screens to route the broadcast result message back to its originator).
const (
	actionScale         = "ecs.scale"
	actionForceDeploy   = "ecs.forceDeploy"
	actionStopTask      = "ecs.stopTask"
	actionUpdateTaskDef = "ecs.updateTaskDef"
)

// writeTimeout bounds every gated ECS mutation.
const writeTimeout = 30 * time.Second

// ecsWriteDoneMsg is the result of a gated ECS mutation. The App broadcasts it
// to every screen, so the originating service/task screen matches on
// action+target to show a status line and refresh its view.
type ecsWriteDoneMsg struct {
	action string // audit action label, e.g. actionScale
	target string // service name or task id
	err    error
}

// noticeFor renders the success line shown after a confirmed mutation.
func noticeFor(action, target string) string {
	switch action {
	case actionScale:
		return "scaled " + target
	case actionForceDeploy:
		return "forced new deployment of " + target
	case actionUpdateTaskDef:
		return "updated task definition for " + target
	case actionStopTask:
		return "stopped task " + target
	default:
		return "done"
	}
}

// familyOf returns the family part of a "family:revision" tail (or the whole
// string when there is no revision suffix).
func familyOf(tail string) string {
	if i := strings.LastIndexByte(tail, ':'); i >= 0 {
		return tail[:i]
	}
	return tail
}

func scaleCmd(client *awsx.EcsClient, cluster, service string, count int32) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return ecsWriteDoneMsg{action: actionScale, target: service, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		defer cancel()
		err := client.UpdateServiceDesiredCount(ctx, cluster, service, count)
		return ecsWriteDoneMsg{action: actionScale, target: service, err: err}
	}
}

func forceDeployCmd(client *awsx.EcsClient, cluster, service string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return ecsWriteDoneMsg{action: actionForceDeploy, target: service, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		defer cancel()
		err := client.ForceNewDeployment(ctx, cluster, service)
		return ecsWriteDoneMsg{action: actionForceDeploy, target: service, err: err}
	}
}

func updateTaskDefCmd(client *awsx.EcsClient, cluster, service, taskDef string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return ecsWriteDoneMsg{action: actionUpdateTaskDef, target: service, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		defer cancel()
		err := client.UpdateServiceTaskDef(ctx, cluster, service, taskDef)
		return ecsWriteDoneMsg{action: actionUpdateTaskDef, target: service, err: err}
	}
}

func stopTaskCmd(client *awsx.EcsClient, cluster, task, reason string) tea.Cmd {
	return func() tea.Msg {
		if client == nil {
			return ecsWriteDoneMsg{action: actionStopTask, target: task, err: fmt.Errorf("aws config not loaded yet")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		defer cancel()
		err := client.StopTask(ctx, cluster, task, reason)
		return ecsWriteDoneMsg{action: actionStopTask, target: task, err: err}
	}
}
