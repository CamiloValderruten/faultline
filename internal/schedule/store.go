package schedule

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Kind string

const (
	KindOnce Kind = "once"
	KindCron Kind = "cron"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusDelivered Status = "delivered"
	StatusCancelled Status = "cancelled"
)

type Task struct {
	ID          string    `json:"id"`
	Kind        Kind      `json:"kind"`
	Title       string    `json:"title"`
	Prompt      string    `json:"prompt"`
	RunAt       time.Time `json:"run_at,omitempty"`
	Cron        string    `json:"cron,omitempty"`
	NextRunAt   time.Time `json:"next_run_at,omitempty"`
	LastRunAt   time.Time `json:"last_run_at,omitempty"`
	Status      Status    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	DeliveredAt time.Time `json:"delivered_at,omitempty"`
	CancelledAt time.Time `json:"cancelled_at,omitempty"`
}

type CreateRequest struct {
	Kind   Kind
	Title  string
	Prompt string
	RunAt  time.Time
	Cron   string

	// Now is optional and exists to make cron scheduling deterministic in tests.
	Now time.Time
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Schedule(req CreateRequest) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Prompt = strings.TrimSpace(req.Prompt)
	req.Cron = strings.TrimSpace(req.Cron)
	if req.Title == "" {
		return Task{}, errors.New("title is required")
	}
	if req.Prompt == "" {
		return Task{}, errors.New("prompt is required")
	}

	task := Task{
		ID:        newID(),
		Kind:      req.Kind,
		Title:     req.Title,
		Prompt:    req.Prompt,
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	switch req.Kind {
	case KindOnce:
		if req.RunAt.IsZero() || req.Cron != "" {
			return Task{}, errors.New("once tasks require run_at and must not include cron")
		}
		task.RunAt = req.RunAt.UTC()
		task.NextRunAt = task.RunAt
	case KindCron:
		if req.Cron == "" || !req.RunAt.IsZero() {
			return Task{}, errors.New("cron tasks require cron and must not include run_at")
		}
		next, err := NextCronRun(req.Cron, now)
		if err != nil {
			return Task{}, err
		}
		task.Cron = req.Cron
		task.NextRunAt = next
	default:
		return Task{}, fmt.Errorf("unsupported kind %q", req.Kind)
	}

	tasks, err := s.loadLocked()
	if err != nil {
		return Task{}, err
	}
	tasks = append(tasks, task)
	if err := s.saveLocked(tasks); err != nil {
		return Task{}, err
	}
	return task, nil
}

func (s *Store) List(status string) ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	out := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		if status == "" || string(task.Status) == status {
			out = append(out, task)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *Store) Cancel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks, err := s.loadLocked()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for i := range tasks {
		if tasks[i].ID == id {
			if tasks[i].Status == StatusCancelled {
				return nil
			}
			tasks[i].Status = StatusCancelled
			tasks[i].CancelledAt = now
			tasks[i].UpdatedAt = now
			return s.saveLocked(tasks)
		}
	}
	return fmt.Errorf("scheduled task %q not found", id)
}

func (s *Store) Due(now time.Time) ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now = now.UTC()
	tasks, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	var due []Task
	changed := false
	for i := range tasks {
		task := &tasks[i]
		if task.Status != StatusActive || task.NextRunAt.IsZero() || task.NextRunAt.After(now) {
			continue
		}
		due = append(due, *task)
		changed = true
		switch task.Kind {
		case KindOnce:
			task.Status = StatusDelivered
			task.DeliveredAt = now
			task.UpdatedAt = now
		case KindCron:
			task.LastRunAt = task.NextRunAt
			next, err := NextCronRun(task.Cron, task.NextRunAt)
			if err != nil {
				return nil, err
			}
			task.NextRunAt = next
			task.UpdatedAt = now
		}
	}
	if changed {
		if err := s.saveLocked(tasks); err != nil {
			return nil, err
		}
	}
	return due, nil
}

func (s *Store) loadLocked() ([]Task, error) {
	if s.path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *Store) saveLocked(tasks []Task) error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	return "task-" + hex.EncodeToString(b[:])
}

type cronSpec struct {
	minute     cronField
	hour       cronField
	dayOfMonth cronField
	month      cronField
	dayOfWeek  cronField
}

type cronField struct {
	all     bool
	allowed map[int]struct{}
}

func NextCronRun(expr string, after time.Time) (time.Time, error) {
	spec, err := parseCron(expr)
	if err != nil {
		return time.Time{}, err
	}
	t := after.UTC().Truncate(time.Minute).Add(time.Minute)
	limit := t.Add(5 * 366 * 24 * time.Hour)
	for !t.After(limit) {
		if spec.matches(t) {
			return t, nil
		}
		t = t.Add(time.Minute)
	}
	return time.Time{}, fmt.Errorf("cron %q has no matching time in the next five years", expr)
}

func parseCron(expr string) (cronSpec, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return cronSpec{}, fmt.Errorf("cron must have exactly 5 fields: minute hour day-of-month month day-of-week")
	}
	minute, err := parseCronField(fields[0], 0, 59, false)
	if err != nil {
		return cronSpec{}, fmt.Errorf("minute: %w", err)
	}
	hour, err := parseCronField(fields[1], 0, 23, false)
	if err != nil {
		return cronSpec{}, fmt.Errorf("hour: %w", err)
	}
	dayOfMonth, err := parseCronField(fields[2], 1, 31, false)
	if err != nil {
		return cronSpec{}, fmt.Errorf("day-of-month: %w", err)
	}
	month, err := parseCronField(fields[3], 1, 12, false)
	if err != nil {
		return cronSpec{}, fmt.Errorf("month: %w", err)
	}
	dayOfWeek, err := parseCronField(fields[4], 0, 7, true)
	if err != nil {
		return cronSpec{}, fmt.Errorf("day-of-week: %w", err)
	}
	return cronSpec{minute: minute, hour: hour, dayOfMonth: dayOfMonth, month: month, dayOfWeek: dayOfWeek}, nil
}

func parseCronField(expr string, minVal, maxVal int, sundaySeven bool) (cronField, error) {
	if expr == "*" {
		return cronField{all: true}, nil
	}
	field := cronField{allowed: map[int]struct{}{}}
	for _, part := range strings.Split(expr, ",") {
		if part == "" {
			return cronField{}, errors.New("empty list element")
		}
		step := 1
		base := part
		if before, after, ok := strings.Cut(part, "/"); ok {
			base = before
			n, err := strconv.Atoi(after)
			if err != nil || n <= 0 {
				return cronField{}, fmt.Errorf("invalid step %q", after)
			}
			step = n
		}
		start, end, err := cronRange(base, minVal, maxVal)
		if err != nil {
			return cronField{}, err
		}
		for n := start; n <= end; n += step {
			if sundaySeven && n == 7 {
				field.allowed[0] = struct{}{}
			} else {
				field.allowed[n] = struct{}{}
			}
		}
	}
	return field, nil
}

func cronRange(expr string, minVal, maxVal int) (int, int, error) {
	if expr == "*" {
		return minVal, maxVal, nil
	}
	if start, end, ok := strings.Cut(expr, "-"); ok {
		a, err := strconv.Atoi(start)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid range start %q", start)
		}
		b, err := strconv.Atoi(end)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid range end %q", end)
		}
		if a > b {
			return 0, 0, fmt.Errorf("range start %d is after end %d", a, b)
		}
		if a < minVal || b > maxVal {
			return 0, 0, fmt.Errorf("range %d-%d outside %d-%d", a, b, minVal, maxVal)
		}
		return a, b, nil
	}
	n, err := strconv.Atoi(expr)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid value %q", expr)
	}
	if n < minVal || n > maxVal {
		return 0, 0, fmt.Errorf("value %d outside %d-%d", n, minVal, maxVal)
	}
	return n, n, nil
}

func (s cronSpec) matches(t time.Time) bool {
	if !s.minute.matches(t.Minute()) || !s.hour.matches(t.Hour()) || !s.month.matches(int(t.Month())) {
		return false
	}
	dom := s.dayOfMonth.matches(t.Day())
	dow := s.dayOfWeek.matches(int(t.Weekday()))
	switch {
	case s.dayOfMonth.all && s.dayOfWeek.all:
		return true
	case s.dayOfMonth.all:
		return dow
	case s.dayOfWeek.all:
		return dom
	default:
		return dom || dow
	}
}

func (f cronField) matches(n int) bool {
	if f.all {
		return true
	}
	_, ok := f.allowed[n]
	return ok
}
