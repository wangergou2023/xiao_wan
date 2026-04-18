package cron

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSchedulerAddListRemovePersists(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	oldWirepodHome := os.Getenv("WIREPOD_HOME")
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("WIREPOD_HOME", tmp); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
		_ = os.Setenv("WIREPOD_HOME", oldWirepodHome)
	}()

	if err := os.MkdirAll(filepath.Join(tmp, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := &Scheduler{}
	s.Init()
	job, err := s.AddJob(Job{
		Name:         "briefing",
		ScheduleType: "every",
		IntervalS:    60,
		Message:      "hello",
		RobotESN:     "0dd1c497",
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(job.ID) == "" {
		t.Fatal("expected job id")
	}
	jobs := s.ListJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].Name != "briefing" {
		t.Fatalf("expected briefing job, got %#v", jobs[0])
	}
	if _, ok, err := s.RemoveJob(job.ID); err != nil || !ok {
		t.Fatalf("expected remove to succeed, ok=%v err=%v", ok, err)
	}
	jobs = s.ListJobs()
	if len(jobs) != 0 {
		t.Fatalf("expected 0 jobs after removal, got %d", len(jobs))
	}
}

func TestSchedulerRejectsPastAtEpoch(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	oldWirepodHome := os.Getenv("WIREPOD_HOME")
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("WIREPOD_HOME", tmp); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chdir(oldWD)
		_ = os.Setenv("WIREPOD_HOME", oldWirepodHome)
	}()
	if err := os.MkdirAll(filepath.Join(tmp, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := &Scheduler{}
	s.Init()
	_, err = s.AddJob(Job{
		Name:         "once",
		ScheduleType: "at",
		AtEpoch:      time.Now().Add(-time.Minute).Unix(),
		Message:      "hello",
		RobotESN:     "0dd1c497",
	})
	if err == nil {
		t.Fatal("expected error for past at_epoch")
	}
}
