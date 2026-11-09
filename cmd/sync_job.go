package cmd

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// maxSyncLogLines caps the progress log kept for /api/sync/status.
const maxSyncLogLines = 500

var errSyncRunning = errors.New("a sync is already running")

// syncStatus is the JSON reply of /api/sync and /api/sync/status.
type syncStatus struct {
	// State is "idle", "running", "done" or "failed".
	State      string              `json:"state"`
	StartDate  string              `json:"start_date,omitempty"`
	EndDate    string              `json:"end_date,omitempty"`
	StartedAt  *time.Time          `json:"started_at,omitempty"`
	FinishedAt *time.Time          `json:"finished_at,omitempty"`
	Log        []string            `json:"log"`
	Result     *activitySyncResult `json:"result,omitempty"`
	Error      string              `json:"error,omitempty"`
	// NeedsAuth is true when a provider's login must be redone with `running-cli setup`.
	NeedsAuth bool `json:"needs_auth"`
}

// syncRunner performs a sync, writing progress to logf.
type syncRunner func(logf func(string, ...interface{})) (*activitySyncResult, error)

// syncJob runs at most one sync at a time in the background and remembers
// the state of the latest one.
type syncJob struct {
	mu     sync.Mutex
	status syncStatus
	done   chan struct{} // closed when the current run finishes
}

func newSyncJob() *syncJob {
	return &syncJob{status: syncStatus{State: "idle", Log: []string{}}}
}

// start launches run in the background. It fails if a sync is already running.
func (j *syncJob) start(start, end time.Time, run syncRunner) (syncStatus, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.status.State == "running" {
		return j.snapshotLocked(), errSyncRunning
	}

	now := time.Now()
	j.status = syncStatus{
		State:     "running",
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
		StartedAt: &now,
		Log:       []string{},
	}
	j.done = make(chan struct{})
	done := j.done

	go func() {
		defer close(done)
		res, err := run(j.logf)

		j.mu.Lock()
		defer j.mu.Unlock()
		finished := time.Now()
		j.status.FinishedAt = &finished
		j.status.Result = res
		if res != nil && len(res.NeedsAuth) > 0 {
			j.status.NeedsAuth = true
		}
		if err != nil {
			j.status.State = "failed"
			j.status.Error = err.Error()
		} else {
			j.status.State = "done"
		}
	}()

	return j.snapshotLocked(), nil
}

// logf appends formatted progress lines to the log.
func (j *syncJob) logf(format string, args ...interface{}) {
	text := strings.TrimRight(fmt.Sprintf(format, args...), "\n")
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, line := range strings.Split(text, "\n") {
		j.status.Log = append(j.status.Log, line)
	}
	if n := len(j.status.Log); n > maxSyncLogLines {
		j.status.Log = j.status.Log[n-maxSyncLogLines:]
	}
}

// snapshot returns a copy of the current status.
func (j *syncJob) snapshot() syncStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.snapshotLocked()
}

func (j *syncJob) snapshotLocked() syncStatus {
	s := j.status
	s.Log = append([]string{}, j.status.Log...)
	return s
}

// wait blocks until the current run (if any) finishes. Used by tests.
func (j *syncJob) wait() {
	j.mu.Lock()
	done := j.done
	j.mu.Unlock()
	if done != nil {
		<-done
	}
}
