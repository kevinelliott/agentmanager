package platform

import (
	"os"
	"os/exec"
	"sync"
	"testing"
)

// TestCachedLookPath_ReturnsSameResultAsLookPath verifies memoization preserves
// the behavior of exec.LookPath.
func TestCachedLookPath_ReturnsSameResultAsLookPath(t *testing.T) {
	resetLookPathCache()

	// Pick an executable that is likely present on the host (sh on Unix, cmd on Windows).
	var name string
	if IsWindows() {
		name = "cmd"
	} else {
		name = "sh"
	}

	want, wantErr := exec.LookPath(name)
	got, gotErr := cachedLookPath(name)

	if (wantErr == nil) != (gotErr == nil) {
		t.Fatalf("cachedLookPath err mismatch: want=%v got=%v", wantErr, gotErr)
	}
	if got != want {
		t.Errorf("cachedLookPath = %q, want %q", got, want)
	}

	// Second call should hit the cache and return the same result.
	got2, gotErr2 := cachedLookPath(name)
	if got2 != got || (gotErr2 == nil) != (gotErr == nil) {
		t.Errorf("cached second call diverged: first=(%q,%v) second=(%q,%v)", got, gotErr, got2, gotErr2)
	}
}

// TestCachedLookPath_InvalidatesOnPathChange ensures entries keyed by PATH+name
// do not leak across PATH mutations.
func TestCachedLookPath_InvalidatesOnPathChange(t *testing.T) {
	resetLookPathCache()

	origPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })

	os.Setenv("PATH", "/nonexistent-dir-xyz")
	if _, err := cachedLookPath("definitely-not-here-xyz123"); err == nil {
		t.Fatalf("expected error with bogus PATH, got nil")
	}

	// Restore PATH — a fresh lookup must be attempted (cache key differs).
	os.Setenv("PATH", origPath)
	var probe string
	if IsWindows() {
		probe = "cmd"
	} else {
		probe = "sh"
	}
	if _, err := cachedLookPath(probe); err != nil {
		t.Errorf("cachedLookPath after PATH restore: unexpected error: %v", err)
	}
}

// TestCachedLookPath_ConcurrentSafe exercises the sync.Map path under -race.
func TestCachedLookPath_ConcurrentSafe(t *testing.T) {
	resetLookPathCache()

	var probe string
	if IsWindows() {
		probe = "cmd"
	} else {
		probe = "sh"
	}

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cachedLookPath(probe)
			_, _ = cachedLookPath("definitely-not-here-xyz123")
		}()
	}
	wg.Wait()
}
