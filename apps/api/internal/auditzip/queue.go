package auditzip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type jobState struct {
	job            AuditZipJob
	tenantID       string
	criteriaHash   string
	idempotencyKey string
	request        AuditZipRequest
	cancel         context.CancelFunc
}

type ConflictErr struct {
	Reason ConflictErrorConflictReason
	JobID  string
}

func (e ConflictErr) Error() string {
	return string(e.Reason)
}

type RateLimitErr struct {
	RetryAfter time.Duration
}

func (e RateLimitErr) Error() string {
	return "rate limited"
}

var ErrNotFound = errors.New("job not found")

type JobQueue struct {
	mu          sync.RWMutex
	jobs        map[string]*jobState
	byKey       map[string]*jobState
	byCriteria  map[string]*jobState
	storage     Storage
	cfg         Config
	workerSlots chan struct{}
}

func NewJobQueue(storage Storage, cfg Config) *JobQueue {
	return &JobQueue{
		jobs:        map[string]*jobState{},
		byKey:       map[string]*jobState{},
		byCriteria:  map[string]*jobState{},
		storage:     storage,
		cfg:         cfg,
		workerSlots: make(chan struct{}, cfg.MaxConcurrentJobs),
	}
}

func (q *JobQueue) Enqueue(ctx context.Context, tenantID, idempotencyKey, criteriaHash string, req AuditZipRequest) (AuditZipJob, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.cfg.MaxQueueDepth > 0 && q.activeCountLocked() >= q.cfg.MaxQueueDepth {
		return AuditZipJob{}, RateLimitErr{RetryAfter: q.cfg.QueueRetryAfter}
	}

	key := fmt.Sprintf("%s:%s", tenantID, idempotencyKey)
	criteriaKey := fmt.Sprintf("%s:%s", tenantID, criteriaHash)

	if existing, ok := q.byKey[key]; ok {
		if existing.criteriaHash == criteriaHash && existing.tenantID == tenantID {
			return cloneJob(existing.job), nil
		}
		return AuditZipJob{}, ConflictErr{Reason: IdempotencyBodyMismatch, JobID: existing.job.JobId.String()}
	}

	if existing, ok := q.byCriteria[criteriaKey]; ok && !isTerminal(existing.job.Status) {
		return AuditZipJob{}, ConflictErr{Reason: DuplicateJob, JobID: existing.job.JobId.String()}
	}

	jobID := uuid.New()
	canCancel := false
	job := AuditZipJob{
		JobId:        jobID,
		Status:       Queued,
		Progress:     0,
		RequestedAt:  time.Now().UTC(),
		RetryCount:   0,
		CriteriaHash: &criteriaHash,
		CanCancel:    &canCancel,
	}
	jobCtx, cancel := context.WithCancel(context.Background())
	state := &jobState{
		job:            job,
		tenantID:       tenantID,
		criteriaHash:   criteriaHash,
		idempotencyKey: idempotencyKey,
		request:        req,
		cancel:         cancel,
	}
	q.jobs[jobID.String()] = state
	q.byKey[key] = state
	q.byCriteria[criteriaKey] = state

	go q.runJob(jobCtx, state)
	return cloneJob(job), nil
}

func (q *JobQueue) Cancel(tenantID, jobID string) (AuditZipJob, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	state, ok := q.jobs[jobID]
	if !ok {
		return AuditZipJob{}, ErrNotFound
	}
	if state.tenantID != tenantID {
		return AuditZipJob{}, ErrNotFound
	}
	if state.job.Status != Running {
		return cloneJob(state.job), ConflictErr{Reason: NotCancelable, JobID: jobID}
	}
	state.cancel()
	now := time.Now().UTC()
	state.job.Status = Canceled
	state.job.FinishedAt = &now
	state.job.Progress = minInt(100, state.job.Progress)
	state.job.Error = &InternalError{Code: "CANCELED", Message: "canceled by user", Retryable: true, CorrId: ""}
	disable := false
	state.job.CanCancel = &disable
	state.job.Result = nil
	q.jobs[jobID] = state
	return cloneJob(state.job), nil
}

func (q *JobQueue) Get(jobID string) (AuditZipJob, string, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	state, ok := q.jobs[jobID]
	if !ok {
		return AuditZipJob{}, "", false
	}
	return cloneJob(state.job), state.tenantID, true
}

func (q *JobQueue) runJob(ctx context.Context, state *jobState) {
	q.workerSlots <- struct{}{}
	defer func() { <-q.workerSlots }()

	start := time.Now().UTC()
	q.updateStatus(state.job.JobId, Running, func(job *AuditZipJob) {
		job.StartedAt = &start
		enable := true
		job.CanCancel = &enable
		job.Progress = 5
	})

	attempt := 0
	for {
		attempt++
		q.setRetryCount(state.job.JobId, attempt-1)
		err := q.processJob(ctx, state)
		if err == nil {
			return
		}
		if errors.Is(err, context.Canceled) {
			return
		}
		if attempt >= q.cfg.MaxRetries {
			q.failJob(state.job.JobId, err)
			return
		}
		backoff := q.cfg.RetryBaseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
	}
}

func (q *JobQueue) processJob(ctx context.Context, state *jobState) error {
	if err := q.bumpProgress(state.job.JobId, 10); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(1 * time.Second):
	}

	if err := q.bumpProgress(state.job.JobId, 50); err != nil {
		return err
	}

	size, err := q.persistArtifacts(ctx, state)
	if err != nil {
		return err
	}

	if err := q.bumpProgress(state.job.JobId, 90); err != nil {
		return err
	}

	expiry := time.Now().UTC().Add(q.cfg.SignURLTTL)
	signed, err := q.storage.GetSignedURL(ctx, q.zipKey(state), q.cfg.SignURLTTL)
	if err != nil {
		return err
	}
	q.completeJob(state.job.JobId, signed, expiry, size)
	return nil
}

func (q *JobQueue) persistArtifacts(ctx context.Context, state *jobState) (int, error) {
	payload := []byte(fmt.Sprintf("audit export %s to %s partner %v", state.request.From.String(), state.request.To.String(), state.request.Partner))
	indexPayload := struct {
		From    string  `json:"from"`
		To      string  `json:"to"`
		Partner *string `json:"partner"`
	}{
		From:    state.request.From.String(),
		To:      state.request.To.String(),
		Partner: state.request.Partner,
	}
	index, _ := json.Marshal(indexPayload)
	hashes := []byte(fmt.Sprintf("%s archive.zip\n%s index.json\n", hashBytes(payload), hashBytes(index)))

	keys := []struct {
		key  string
		body []byte
		ct   string
	}{
		{q.zipKey(state), payload, "application/zip"},
		{q.indexKey(state), index, "application/json"},
		{q.hashKey(state), hashes, "text/plain"},
	}
	for _, obj := range keys {
		if err := q.storage.PutObject(ctx, obj.key, obj.body, obj.ct); err != nil {
			return 0, err
		}
	}
	go func() {
		timer := time.NewTimer(q.cfg.RetentionPeriod)
		defer timer.Stop()
		select {
		case <-timer.C:
			_ = q.storage.DeleteObject(context.Background(), q.zipKey(state))
			_ = q.storage.DeleteObject(context.Background(), q.indexKey(state))
			_ = q.storage.DeleteObject(context.Background(), q.hashKey(state))
		case <-ctx.Done():
		}
	}()
	return len(payload), nil
}

func (q *JobQueue) completeJob(jobID openapiUUID, signedURL string, expiresAt time.Time, size int) {
	now := time.Now().UTC()
	q.updateStatus(jobID, Succeeded, func(job *AuditZipJob) {
		job.FinishedAt = &now
		job.Progress = 100
		job.Result = &AuditZipResult{SignedUrl: signedURL, ExpiresAt: expiresAt, Size: size}
		disable := false
		job.CanCancel = &disable
		job.Error = nil
	})
}

func (q *JobQueue) failJob(jobID openapiUUID, err error) {
	now := time.Now().UTC()
	q.updateStatus(jobID, Failed, func(job *AuditZipJob) {
		job.FinishedAt = &now
		disable := false
		job.CanCancel = &disable
		job.Result = nil
		job.Error = &InternalError{Code: "INTERNAL_ERROR", Message: err.Error(), Retryable: true}
	})
}

func (q *JobQueue) bumpProgress(jobID openapiUUID, progress int) error {
	return q.updateWithErr(jobID, func(job *AuditZipJob) error {
		if job.Status == Canceled {
			return context.Canceled
		}
		if progress > job.Progress {
			job.Progress = progress
		}
		return nil
	})
}

func (q *JobQueue) setRetryCount(jobID openapiUUID, retries int) {
	q.updateStatus(jobID, Running, func(job *AuditZipJob) {
		job.RetryCount = retries
	})
}

func (q *JobQueue) updateStatus(jobID openapiUUID, status AuditZipJobStatus, mutate func(job *AuditZipJob)) {
	_ = q.updateWithErr(jobID, func(job *AuditZipJob) error {
		job.Status = status
		mutate(job)
		return nil
	})
}

func (q *JobQueue) updateWithErr(jobID openapiUUID, mutate func(job *AuditZipJob) error) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	state, ok := q.jobs[jobID.String()]
	if !ok {
		return ErrNotFound
	}
	if err := mutate(&state.job); err != nil {
		return err
	}
	q.jobs[jobID.String()] = state
	return nil
}

func (q *JobQueue) zipKey(state *jobState) string {
	return fmt.Sprintf("%s/%s/%s/archive.zip", q.cfg.S3Bucket, state.tenantID, state.job.JobId)
}

func (q *JobQueue) indexKey(state *jobState) string {
	return fmt.Sprintf("%s/%s/%s/index.json", q.cfg.S3Bucket, state.tenantID, state.job.JobId)
}

func (q *JobQueue) hashKey(state *jobState) string {
	return fmt.Sprintf("%s/%s/%s/hashes.txt", q.cfg.S3Bucket, state.tenantID, state.job.JobId)
}

func cloneJob(job AuditZipJob) AuditZipJob {
	clone := job
	if job.CriteriaHash != nil {
		ch := *job.CriteriaHash
		clone.CriteriaHash = &ch
	}
	if job.CanCancel != nil {
		cc := *job.CanCancel
		clone.CanCancel = &cc
	}
	if job.Result != nil {
		res := *job.Result
		clone.Result = &res
	}
	if job.Error != nil {
		e := *job.Error
		clone.Error = &e
	}
	if job.StartedAt != nil {
		t := *job.StartedAt
		clone.StartedAt = &t
	}
	if job.FinishedAt != nil {
		t := *job.FinishedAt
		clone.FinishedAt = &t
	}
	return clone
}

func isTerminal(status AuditZipJobStatus) bool {
	return status == Succeeded || status == Failed || status == Canceled
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (q *JobQueue) activeCountLocked() int {
	count := 0
	for _, state := range q.jobs {
		if !isTerminal(state.job.Status) {
			count++
		}
	}
	return count
}

type openapiUUID = openapi_types.UUID

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
