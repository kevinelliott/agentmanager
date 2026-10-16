package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kevinelliott/agentmanager/pkg/agent"
	"github.com/kevinelliott/agentmanager/pkg/catalog"
	"github.com/kevinelliott/agentmanager/pkg/storage"
)

type contextDetector func(context.Context, []catalog.AgentDef) ([]*agent.Installation, error)

func (f contextDetector) DetectAll(ctx context.Context, defs []catalog.AgentDef) ([]*agent.Installation, error) {
	return f(ctx, defs)
}

type contextLockStore struct {
	storage.Store
	acquire func(context.Context, string) (func(), error)
}

func (s contextLockStore) AcquireRefreshLock(ctx context.Context, key string) (func(), error) {
	return s.acquire(ctx, key)
}

func TestDetectionPreservesCallerDeadline(t *testing.T) {
	for _, budget := range []time.Duration{0, 5 * time.Second, 120 * time.Second, 5 * time.Minute} {
		t.Run(budget.String(), func(t *testing.T) {
			ctx := t.Context()
			var expected time.Time
			if budget > 0 {
				expected = time.Now().Add(budget)
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, expected)
				defer cancel()
			}
			started := time.Now()
			checkDeadline := func(ctx context.Context) {
				t.Helper()
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("detection has no deadline")
				}
				if budget > 0 {
					if !deadline.Equal(expected) {
						t.Errorf("caller deadline changed: got %v, want %v", deadline, expected)
					}
				} else if deadline.Before(started.Add(30*time.Second)) || deadline.After(time.Now().Add(30*time.Second)) {
					t.Errorf("unbounded caller did not receive the default scan budget: %v", deadline)
				}
			}
			locked, detected := false, false
			store := contextLockStore{Store: &fakeStore{}, acquire: func(ctx context.Context, _ string) (func(), error) {
				locked = true
				checkDeadline(ctx)
				return func() {}, nil
			}}
			det := contextDetector(func(ctx context.Context, _ []catalog.AgentDef) ([]*agent.Installation, error) {
				detected = true
				checkDeadline(ctx)
				return []*agent.Installation{makeInstallation("a")}, nil
			})
			p := New(defaultConfig(), fakePlatform{}, store, &fakeCatalog{}, det, newFakeFetcher())
			if _, err := p.DetectAndCheckVersions(ctx, Options{SkipVersionCheck: true}); err != nil {
				t.Fatal(err)
			}
			if !locked || !detected {
				t.Fatal("expected both coordination and detection to run")
			}
		})
	}
}

func TestDetectionPropagatesCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	store := &fakeStore{}
	det := contextDetector(func(ctx context.Context, _ []catalog.AgentDef) ([]*agent.Installation, error) {
		cancel()
		<-ctx.Done()
		return nil, ctx.Err()
	})
	p := New(defaultConfig(), fakePlatform{}, store, &fakeCatalog{}, det, newFakeFetcher())
	if _, err := p.DetectAndCheckVersions(ctx, Options{SkipVersionCheck: true}); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected caller cancellation, got %v", err)
	}
	if store.saveCount != 0 {
		t.Fatal("canceled detection was cached")
	}
}
