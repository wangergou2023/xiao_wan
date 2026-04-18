package cron

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/logger"
	robotpkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/ttr/robot"
	workspacepkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/workspace"
)

type Job struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ScheduleType string `json:"schedule_type"`
	IntervalS    int64  `json:"interval_s,omitempty"`
	AtEpoch      int64  `json:"at_epoch,omitempty"`
	Message      string `json:"message"`
	RobotESN     string `json:"robot_esn,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	NextRunAt    int64  `json:"next_run_at"`
	LastRunAt    int64  `json:"last_run_at,omitempty"`
}

type snapshot struct {
	Jobs []Job `json:"jobs"`
}

type Scheduler struct {
	mu      sync.Mutex
	jobs    map[string]Job
	started bool
	wakeCh  chan struct{}
	stopCh  chan struct{}
}

var global = &Scheduler{}

func Init() {
	global.Init()
}

func AddJob(job Job) (Job, error) {
	return global.AddJob(job)
}

func ListJobs() []Job {
	return global.ListJobs()
}

func RemoveJob(id string) (Job, bool, error) {
	return global.RemoveJob(id)
}

func (s *Scheduler) Init() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	s.jobs = map[string]Job{}
	s.wakeCh = make(chan struct{}, 1)
	s.stopCh = make(chan struct{})
	s.loadLocked()
	s.started = true
	go s.loop()
}

func (s *Scheduler) AddJob(job Job) (Job, error) {
	s.Init()
	s.mu.Lock()
	defer s.mu.Unlock()
	job.Name = strings.TrimSpace(job.Name)
	job.ScheduleType = strings.TrimSpace(strings.ToLower(job.ScheduleType))
	job.Message = strings.TrimSpace(job.Message)
	job.RobotESN = strings.TrimSpace(job.RobotESN)
	if job.Name == "" {
		return Job{}, fmt.Errorf("name is required")
	}
	if job.Message == "" {
		return Job{}, fmt.Errorf("message is required")
	}
	if job.RobotESN == "" {
		return Job{}, fmt.Errorf("robot_esn is required")
	}
	if job.ScheduleType != "every" && job.ScheduleType != "at" {
		return Job{}, fmt.Errorf("schedule_type must be every or at")
	}
	now := time.Now().Unix()
	job.ID = newJobIDLocked(s.jobs)
	job.CreatedAt = now
	job.LastRunAt = 0
	if job.ScheduleType == "every" {
		if job.IntervalS <= 0 {
			return Job{}, fmt.Errorf("interval_s must be greater than 0")
		}
		job.NextRunAt = now + job.IntervalS
		job.AtEpoch = 0
	} else {
		if job.AtEpoch <= now {
			return Job{}, fmt.Errorf("at_epoch must be in the future")
		}
		job.NextRunAt = job.AtEpoch
		job.IntervalS = 0
	}
	s.jobs[job.ID] = job
	if err := s.saveLocked(); err != nil {
		delete(s.jobs, job.ID)
		return Job{}, err
	}
	s.signalLocked()
	return job, nil
}

func (s *Scheduler) ListJobs() []Job {
	s.Init()
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs := make([]Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].NextRunAt == jobs[j].NextRunAt {
			return jobs[i].ID < jobs[j].ID
		}
		return jobs[i].NextRunAt < jobs[j].NextRunAt
	})
	return jobs
}

func (s *Scheduler) RemoveJob(id string) (Job, bool, error) {
	s.Init()
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.TrimSpace(id)
	job, ok := s.jobs[id]
	if !ok {
		return Job{}, false, nil
	}
	delete(s.jobs, id)
	if err := s.saveLocked(); err != nil {
		s.jobs[id] = job
		return Job{}, false, err
	}
	s.signalLocked()
	return job, true, nil
}

func (s *Scheduler) loop() {
	for {
		wait := s.nextWaitDuration()
		var timer <-chan time.Time
		if wait > 0 {
			timer = time.After(wait)
		}
		select {
		case <-s.stopCh:
			return
		case <-s.wakeCh:
			continue
		case <-timer:
			s.runDue()
		}
	}
}

func (s *Scheduler) nextWaitDuration() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.jobs) == 0 {
		return time.Hour
	}
	now := time.Now().Unix()
	minWait := int64(3600)
	for _, job := range s.jobs {
		if job.NextRunAt <= now {
			return 0
		}
		if delta := job.NextRunAt - now; delta < minWait {
			minWait = delta
		}
	}
	if minWait < 1 {
		minWait = 1
	}
	return time.Duration(minWait) * time.Second
}

func (s *Scheduler) runDue() {
	var due []Job
	now := time.Now().Unix()
	s.mu.Lock()
	for _, job := range s.jobs {
		if job.NextRunAt <= now {
			due = append(due, job)
		}
	}
	s.mu.Unlock()
	if len(due) == 0 {
		return
	}
	for _, job := range due {
		logger.Println("Cron job firing: id=" + job.ID + " name=" + job.Name + " esn=" + job.RobotESN)
		_ = robotpkg.KGSim(job.RobotESN, job.Message)
		s.markRun(job, time.Now().Unix())
	}
}

func (s *Scheduler) markRun(job Job, firedAt int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.jobs[job.ID]
	if !ok {
		return
	}
	current.LastRunAt = firedAt
	if current.ScheduleType == "every" {
		next := firedAt + current.IntervalS
		if next <= firedAt {
			next = firedAt + 1
		}
		current.NextRunAt = next
		s.jobs[job.ID] = current
	} else {
		delete(s.jobs, job.ID)
	}
	if err := s.saveLocked(); err != nil {
		logger.Println("Cron save failed: " + err.Error())
	}
}

func (s *Scheduler) loadLocked() {
	path := jobsPath()
	if strings.TrimSpace(path) == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var snap snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		logger.Println("Cron load failed: " + err.Error())
		return
	}
	for _, job := range snap.Jobs {
		if strings.TrimSpace(job.ID) == "" {
			continue
		}
		s.jobs[job.ID] = job
	}
}

func (s *Scheduler) saveLocked() error {
	path := jobsPath()
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("cron workspace path unavailable")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	jobs := make([]Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].NextRunAt == jobs[j].NextRunAt {
			return jobs[i].ID < jobs[j].ID
		}
		return jobs[i].NextRunAt < jobs[j].NextRunAt
	})
	data, err := json.MarshalIndent(snapshot{Jobs: jobs}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s *Scheduler) signalLocked() {
	select {
	case s.wakeCh <- struct{}{}:
	default:
	}
}

func jobsPath() string {
	return workspacepkg.ResolveWritableDocPath(filepath.Join("cron", "jobs.json"))
}

func newJobIDLocked(existing map[string]Job) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	for {
		buf := make([]byte, 8)
		for i := range buf {
			buf[i] = letters[rand.Intn(len(letters))]
		}
		id := string(buf)
		if _, ok := existing[id]; !ok {
			return id
		}
	}
}
