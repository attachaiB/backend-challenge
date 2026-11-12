package worker

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/attachaiB/backend-challenge/internal/service"
)

// countingService implements only Count. The embedded interface is nil,
// so calling any other method would panic and fail the test.
type countingService struct {
	service.UserService

	calls atomic.Int32
	err   error
}

func (c *countingService) Count(context.Context) (int64, error) {
	c.calls.Add(1)
	return 7, c.err
}

// safeBuffer lets the worker goroutine and the test use the log output together.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func captureLog(t *testing.T) *safeBuffer {
	t.Helper()

	buf := &safeBuffer{}
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	return buf
}

func runFor(svc service.UserService, d time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	RunUserCounter(ctx, svc, 10*time.Millisecond)
}

func TestUserCounterLogsTotalOnEveryTick(t *testing.T) {
	buf := captureLog(t)
	svc := &countingService{}

	runFor(svc, 55*time.Millisecond)

	if svc.calls.Load() < 3 {
		t.Errorf("Count called %d times, want at least 3", svc.calls.Load())
	}

	if !strings.Contains(buf.String(), "total users: 7") {
		t.Errorf("log %q does not contain the total", buf.String())
	}
}

func TestUserCounterStopsWhenContextIsCancelled(t *testing.T) {
	captureLog(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		RunUserCounter(ctx, &countingService{}, time.Hour)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after the context was cancelled")
	}
}

func TestUserCounterKeepsRunningAfterAnError(t *testing.T) {
	buf := captureLog(t)
	svc := &countingService{err: errors.New("mongo is down")}

	runFor(svc, 35*time.Millisecond)

	if svc.calls.Load() < 2 {
		t.Errorf("Count called %d times, want the worker to keep trying", svc.calls.Load())
	}

	if !strings.Contains(buf.String(), "mongo is down") {
		t.Errorf("log %q does not contain the error", buf.String())
	}
}
