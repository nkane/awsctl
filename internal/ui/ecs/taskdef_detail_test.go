package ecs

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	tea "github.com/charmbracelet/bubbletea"
	awsx "github.com/nkane/awsctl/internal/aws"
)

func runeKey(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func sampleTaskDef() *ecstypes.TaskDefinition {
	return &ecstypes.TaskDefinition{
		Family:                  aws.String("api"),
		Revision:                7,
		Status:                  ecstypes.TaskDefinitionStatusActive,
		Cpu:                     aws.String("256"),
		Memory:                  aws.String("512"),
		NetworkMode:             ecstypes.NetworkModeAwsvpc,
		RequiresCompatibilities: []ecstypes.Compatibility{ecstypes.CompatibilityFargate},
		ContainerDefinitions: []ecstypes.ContainerDefinition{{
			Name:        aws.String("web"),
			Image:       aws.String("nginx:1.27"),
			Essential:   aws.Bool(true),
			Memory:      aws.Int32(256),
			Environment: []ecstypes.KeyValuePair{{Name: aws.String("LOG_LEVEL"), Value: aws.String("info")}},
			Secrets:     []ecstypes.Secret{{Name: aws.String("TOKEN"), ValueFrom: aws.String("arn:secret")}},
		}},
	}
}

func TestTaskDefDescribeRenders(t *testing.T) {
	m := NewTaskDefDescribe(awsx.NewEcsClient(&awsx.Config{}), "api")
	m.SetSize(120, 40)
	m, _ = m.Update(taskDefDescribeLoadedMsg{family: "api", td: sampleTaskDef()})

	v := m.View()
	for _, want := range []string{"task-def: api", "revision", "FARGATE", "nginx:1.27", "LOG_LEVEL", "TOKEN"} {
		if !strings.Contains(v, want) {
			t.Fatalf("task-def describe view missing %q; got:\n%s", want, v)
		}
	}
}

func TestTaskDefDescribeJSONToggle(t *testing.T) {
	m := NewTaskDefDescribe(awsx.NewEcsClient(&awsx.Config{}), "api")
	m.SetSize(120, 40)
	m, _ = m.Update(taskDefDescribeLoadedMsg{family: "api", td: sampleTaskDef()})

	// Toggle to raw JSON with 'J'.
	m, _ = m.Update(runeKey("J"))
	v := m.View()
	if !strings.Contains(v, "[raw json]") {
		t.Fatalf("expected raw-json badge after toggle; got:\n%s", v)
	}
	if !strings.Contains(v, "\"ContainerDefinitions\"") {
		t.Fatalf("expected JSON content after toggle; got:\n%s", v)
	}
	// The full JSON (independent of viewport clipping) carries every field.
	if !strings.Contains(renderTaskDefJSON(sampleTaskDef()), "\"Family\": \"api\"") {
		t.Fatal("renderTaskDefJSON should include the Family field")
	}
}

// TestTaskDefDrillOpensDescribe verifies enter on a family builds the describe
// (#50 -> #51 drill).
func TestTaskDefDrillOpensDescribe(t *testing.T) {
	tl := &taskDefListScreen{m: NewTaskDefList(awsx.NewEcsClient(&awsx.Config{}))}
	tl.m, _ = tl.m.Update(taskDefsLoadedMsg{families: []awsx.TaskDefFamilySummary{{Family: "api", Revision: "7"}}})

	scr := tl.OpenDescribe(&awsx.Config{})
	if scr == nil || scr.Title() != "describe" {
		t.Fatalf("OpenDescribe should yield a 'describe' screen, got %v", scr)
	}
}
