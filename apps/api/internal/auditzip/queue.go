package auditzip

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type JobQueue struct {
	mu      sync.RWMutex
	jobs    map[string]AuditZipJob
	storage Storage
	cfg     Config
}

func NewJobQueue(storage Storage, cfg Config) *JobQueue {
	return &JobQueue{
		jobs:    map[string]AuditZipJob{},
		storage: storage,
		cfg:     cfg,
	}
}

func (q *JobQueue) Enqueue(ctx context.Context, tenantID string, req AuditZipRequest) AuditZipJob {
	jobID := newID()
	job := AuditZipJob{
		JobID:       jobID,
		Status:      "queued",
		RequestedAt: time.Now().UTC(),
	}
	q.mu.Lock()
	q.jobs[jobID] = job
	q.mu.Unlock()

	go q.runJob(ctx, tenantID, jobID, req)
	return job
}

func (q *JobQueue) runJob(ctx context.Context, tenantID, jobID string, req AuditZipRequest) {
	q.setStatus(jobID, "running", nil, nil, nil)
	select {
	case <-ctx.Done():
		q.setStatus(jobID, "canceled", nil, nil, &InternalError{Code: "CANCELED", Message: "context canceled", Retryable: true})
		return
	case <-time.After(minDuration(3*time.Second, q.cfg.JobMaxDuration)):
		zipKey := fmt.Sprintf("%s/audit/%s/export.zip", tenantID, jobID)
		body := []byte(fmt.Sprintf("audit export %s to %s partner %v", req.From, req.To, req.Partner))
		_ = q.storage.PutObject(context.Background(), zipKey, body, "application/zip")
		signed, _ := q.storage.GetSignedURL(context.Background(), zipKey, q.cfg.SignURLTTL)
		now := time.Now().UTC()
		result := &AuditZipResult{SignedURL: signed, Size: len(body)}
		q.setStatus(jobID, "succeeded", &now, result, nil)
	}
}

func (q *JobQueue) setStatus(jobID, status string, completedAt *time.Time, result *AuditZipResult, err *InternalError) {
	q.mu.Lock()
	defer q.mu.Unlock()
	job, ok := q.jobs[jobID]
	if !ok {
		return
	}
	job.Status = status
	if completedAt != nil {
		job.CompletedAt = completedAt
	}
	if result != nil {
		job.Result = result
		job.Error = nil
	}
	if err != nil {
		job.Error = err
		job.Result = nil
	}
	q.jobs[jobID] = job
}

func (q *JobQueue) Get(jobID string) (AuditZipJob, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	job, ok := q.jobs[jobID]
	return job, ok
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
