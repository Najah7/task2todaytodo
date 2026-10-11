package usecase

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Najah7/task2todaytodo/internal/telemetry"
)

var ErrLeaseLost = errors.New("operation lease lost")

type AsyncRunner struct {
	repo     OperationRepository
	executor BackfillExecutor
	process  context.Context
	cancel   context.CancelFunc
	lease    time.Duration
	interval time.Duration
	logger   telemetry.Logger
	mu       sync.Mutex
	closed   bool
	wg       sync.WaitGroup
}

func NewAsyncRunner(repo OperationRepository, executor BackfillExecutor, process context.Context, lease, heartbeatInterval time.Duration, loggers ...telemetry.Logger) *AsyncRunner {
	if process == nil {
		process = context.Background()
	}
	runContext, cancel := context.WithCancel(process)
	if lease <= 0 {
		lease = 30 * time.Second
	}
	if heartbeatInterval <= 0 || heartbeatInterval >= lease {
		heartbeatInterval = lease / 3
	}
	var logger telemetry.Logger
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return &AsyncRunner{repo: repo, executor: executor, process: runContext, cancel: cancel, lease: lease, interval: heartbeatInterval, logger: telemetry.OrNop(logger)}
}

func (r *AsyncRunner) StartOperation(id, token, actorID string, cutoff time.Time) {
	r.mu.Lock()
	if r.closed || r.process.Err() != nil {
		r.mu.Unlock()
		return
	}
	r.wg.Add(1)
	r.mu.Unlock()
	go func() { defer r.wg.Done(); r.execute(id, token, actorID, cutoff) }()
}

func (r *AsyncRunner) execute(id, token, actorID string, cutoff time.Time) {
	ctx, cancel := context.WithCancel(r.process)
	defer cancel()
	stopHeartbeat := make(chan struct{})
	heartbeatDone := make(chan struct{})
	var leaseLost atomic.Bool
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-stopHeartbeat:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				heartbeatCtx, cancelHeartbeat := context.WithTimeout(ctx, r.repositoryCallTimeout())
				ok, err := r.repo.Heartbeat(heartbeatCtx, id, token, r.lease)
				cancelHeartbeat()
				if err != nil {
					if !errors.Is(err, ErrLeaseLost) {
						r.logFailure(ctx, "AdminBackfillRunner.Heartbeat", err)
					}
					if ctx.Err() != nil {
						// Process shutdown already canceled the executor. Let the
						// interrupted failure write verify whether this lease is live.
						return
					}
					// We no longer know whether this process still owns the lease.
					// Never publish success after losing the ability to renew it.
					leaseLost.Store(true)
					cancel()
					return
				}
				if !ok {
					leaseLost.Store(true)
					cancel()
					return
				}
			}
		}
	}()

	var latestProcessed, latestGenerated int64
	report := func(processed, generated int) error {
		latestProcessed, latestGenerated = int64(processed), int64(generated)
		progressCtx, cancelProgress := context.WithTimeout(ctx, r.repositoryCallTimeout())
		ok, err := r.repo.Progress(progressCtx, id, token, latestProcessed, latestGenerated, r.lease)
		cancelProgress()
		if err != nil {
			if !errors.Is(err, ErrLeaseLost) {
				r.logFailure(ctx, "AdminBackfillRunner.Progress", err)
			}
			if ctx.Err() == nil {
				leaseLost.Store(true)
				cancel()
			}
			return err
		}
		if !ok {
			leaseLost.Store(true)
			cancel()
			return ErrLeaseLost
		}
		return nil
	}
	result, err := r.executor.Execute(ctx, actorID, cutoff, report)
	if err == nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	close(stopHeartbeat)
	<-heartbeatDone
	if err != nil {
		if !errors.Is(err, ErrLeaseLost) {
			r.logFailure(ctx, "AdminBackfillRunner.Execute", err)
		}
		if leaseLost.Load() {
			return
		}
		code, message := "backfill_failed", "Schedule backfill failed. Run job again to resume from saved progress."
		if errors.Is(err, ErrLeaseLost) {
			return
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			code, message = "interrupted", "Operation interrupted. Run job again to resume from saved progress."
		}
		writeCtx, cancelWrite := context.WithTimeout(context.Background(), 5*time.Second)
		if failErr := r.repo.Fail(writeCtx, id, token, code, message); failErr != nil && !errors.Is(failErr, ErrLeaseLost) {
			r.logFailure(writeCtx, "AdminBackfillRunner.Fail", failErr)
		}
		cancelWrite()
		return
	}
	if result.ProcessedBlueprints > latestProcessed {
		latestProcessed = result.ProcessedBlueprints
	}
	if result.GeneratedSchedules > latestGenerated {
		latestGenerated = result.GeneratedSchedules
	}
	if leaseLost.Load() {
		return
	}
	writeCtx, cancelWrite := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelWrite()
	if err := r.repo.Succeed(writeCtx, id, token, latestProcessed, latestGenerated); err != nil {
		if !errors.Is(err, ErrLeaseLost) {
			r.logFailure(writeCtx, "AdminBackfillRunner.Succeed", err)
		}
	}
}

func (r *AsyncRunner) repositoryCallTimeout() time.Duration {
	timeout := r.interval
	if leaseTimeout := r.lease / 3; leaseTimeout > 0 && leaseTimeout < timeout {
		timeout = leaseTimeout
	}
	if timeout <= 0 {
		return time.Second
	}
	return timeout
}

func (r *AsyncRunner) logFailure(ctx context.Context, operation string, err error) {
	if err == nil || telemetry.IsRoutineError(err) {
		return
	}
	fields := append([]any{"operation", operation}, telemetry.ErrorFields(err)...)
	r.logger.Error(ctx, "admin backfill operation failed", fields...)
}

// Shutdown cancels owned work and waits before the database pool is closed.
func (r *AsyncRunner) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	r.cancel()
	finished := make(chan struct{})
	go func() { r.wg.Wait(); close(finished) }()
	select {
	case <-finished:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var _ Runner = (*AsyncRunner)(nil)
