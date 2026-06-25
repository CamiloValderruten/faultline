package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CamiloValderruten/faultline/internal/llm"
	"github.com/CamiloValderruten/faultline/internal/schedule"
)

func TestToolDefsAdvertisesScheduleToolsWhenConfigured(t *testing.T) {
	te := New(Deps{
		Logger:    silentTestLogger(),
		Scheduler: schedule.NewStore(filepath.Join(t.TempDir(), "scheduled-tasks.json")),
	})

	names := toolDefNames(te.ToolDefs())
	for _, name := range []string{"schedule_task", "list_scheduled_tasks", "cancel_scheduled_task"} {
		if !names[name] {
			t.Fatalf("expected %s to be advertised", name)
		}
	}
}

func TestExecuteScheduleTaskCreatesOneShotTask(t *testing.T) {
	store := schedule.NewStore(filepath.Join(t.TempDir(), "scheduled-tasks.json"))
	te := New(Deps{Logger: silentTestLogger(), Scheduler: store})

	got := te.Execute(context.Background(), llm.ToolCall{
		Function: llm.FunctionCall{
			Name:      "schedule_task",
			Arguments: `{"kind":"once","title":"Retry Slack","prompt":"Retry discovery.","run_at":"2026-05-04T12:00:00Z"}`,
		},
	})
	if strings.HasPrefix(got, "Error:") {
		t.Fatalf("schedule_task returned error: %s", got)
	}

	tasks, err := store.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("task count = %d, want 1", len(tasks))
	}
	if tasks[0].Kind != schedule.KindOnce || tasks[0].Title != "Retry Slack" {
		t.Fatalf("task = %+v, want one-shot Retry Slack", tasks[0])
	}
}

func TestExecuteScheduleTaskRequiresExactlyOneSchedule(t *testing.T) {
	store := schedule.NewStore(filepath.Join(t.TempDir(), "scheduled-tasks.json"))
	te := New(Deps{Logger: silentTestLogger(), Scheduler: store})

	got := te.Execute(context.Background(), llm.ToolCall{
		Function: llm.FunctionCall{
			Name:      "schedule_task",
			Arguments: `{"kind":"once","title":"Bad","prompt":"Bad","run_at":"2026-05-04T12:00:00Z","cron":"* * * * *"}`,
		},
	})
	if !strings.Contains(got, "exactly one of run_at or cron") {
		t.Fatalf("schedule_task = %q, want exactly-one validation", got)
	}
}

func TestExecuteListScheduledTasksReturnsJSON(t *testing.T) {
	store := schedule.NewStore(filepath.Join(t.TempDir(), "scheduled-tasks.json"))
	te := New(Deps{Logger: silentTestLogger(), Scheduler: store})
	_, err := store.Schedule(schedule.CreateRequest{
		Kind:   schedule.KindCron,
		Title:  "Hourly",
		Prompt: "Check status.",
		Cron:   "0 * * * *",
		Now:    time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	got := te.Execute(context.Background(), llm.ToolCall{
		Function: llm.FunctionCall{Name: "list_scheduled_tasks", Arguments: `{}`},
	})

	var tasks []schedule.Task
	if err := json.Unmarshal([]byte(got), &tasks); err != nil {
		t.Fatalf("list_scheduled_tasks returned invalid JSON: %v\n%s", err, got)
	}
	if len(tasks) != 1 || tasks[0].Title != "Hourly" {
		t.Fatalf("tasks = %+v, want Hourly task", tasks)
	}
}
