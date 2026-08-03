package schedule

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreDueDeliversOneShotOnce(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "scheduled-tasks.json"))
	runAt := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

	task, err := store.Schedule(CreateRequest{
		Kind:   KindOnce,
		Title:  "Retry MCP discovery",
		Prompt: "Retry mcp_discover_tools for slack.",
		RunAt:  runAt,
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	due, err := store.Due(runAt.Add(time.Second))
	if err != nil {
		t.Fatalf("Due: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("due count = %d, want 1", len(due))
	}
	if due[0].ID != task.ID {
		t.Fatalf("due task ID = %q, want %q", due[0].ID, task.ID)
	}

	due, err = store.Due(runAt.Add(2 * time.Second))
	if err != nil {
		t.Fatalf("Due second call: %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("due count after delivery = %d, want 0", len(due))
	}

	tasks, err := store.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if tasks[0].Status != StatusDelivered {
		t.Fatalf("status = %q, want %q", tasks[0].Status, StatusDelivered)
	}
}

func TestStoreDueReschedulesCronTask(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "scheduled-tasks.json"))
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

	task, err := store.Schedule(CreateRequest{
		Kind:   KindCron,
		Title:  "Quarter-hour check",
		Prompt: "Check status.",
		Cron:   "*/15 * * * *",
		Now:    now,
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	if !task.NextRunAt.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("NextRunAt = %s, want %s", task.NextRunAt, now.Add(15*time.Minute))
	}

	due, err := store.Due(now.Add(15 * time.Minute))
	if err != nil {
		t.Fatalf("Due: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("due count = %d, want 1", len(due))
	}

	tasks, err := store.List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if tasks[0].Status != StatusActive {
		t.Fatalf("status = %q, want active", tasks[0].Status)
	}
	if !tasks[0].NextRunAt.Equal(now.Add(30 * time.Minute)) {
		t.Fatalf("NextRunAt after delivery = %s, want %s", tasks[0].NextRunAt, now.Add(30*time.Minute))
	}
}

func TestStoreCancelPreventsDelivery(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "scheduled-tasks.json"))
	runAt := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

	task, err := store.Schedule(CreateRequest{
		Kind:   KindOnce,
		Title:  "Canceled",
		Prompt: "Do not run.",
		RunAt:  runAt,
	})
	if err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	if err := store.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	due, err := store.Due(runAt.Add(time.Second))
	if err != nil {
		t.Fatalf("Due: %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("due count = %d, want 0", len(due))
	}
}

func TestParseCronRejectsUnsupportedShape(t *testing.T) {
	if _, err := NextCronRun("0 0 1 1 1 1", time.Now()); err == nil {
		t.Fatal("expected six-field cron to be rejected")
	}
}
