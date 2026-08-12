package xmluimcp

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// makeValidCachedRepo creates a directory shaped like a complete cached repo.
func makeValidCachedRepo(t *testing.T, reposDir, tag string) string {
	t.Helper()
	repoDir := filepath.Join(reposDir, tag)
	for _, sub := range []string{"xmlui", "docs"} {
		if err := os.MkdirAll(filepath.Join(repoDir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return repoDir
}

// stubArchive writes a zip whose top-level dir contains xmlui/ and docs/,
// matching what downloadAndInstallRepo expects to extract.
func stubArchive(t *testing.T, destPath string) {
	t.Helper()
	out, err := os.Create(destPath)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	w := zip.NewWriter(out)
	for _, name := range []string{"xmlui-top/xmlui/placeholder.txt", "xmlui-top/docs/placeholder.txt"} {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func stubFetchers(t *testing.T, fetch func() (string, string, error), download func(url, dest string) error, done func(tag string, err error)) {
	t.Helper()
	prevFetch, prevDownload, prevDone := fetchLatestXMLUITag, downloadArchive, onBackgroundRefreshDone
	if fetch != nil {
		fetchLatestXMLUITag = fetch
	}
	if download != nil {
		downloadArchive = download
	}
	if done != nil {
		onBackgroundRefreshDone = done
	}
	t.Cleanup(func() {
		fetchLatestXMLUITag = prevFetch
		downloadArchive = prevDownload
		onBackgroundRefreshDone = prevDone
	})
}

func TestPinnedVersionServedFromCache(t *testing.T) {
	reposDir := t.TempDir()
	want := makeValidCachedRepo(t, reposDir, "xmlui@0.14.0")
	stubFetchers(t, func() (string, string, error) {
		t.Fatal("latest fetch must not run for pinned version")
		return "", "", nil
	}, func(url, dest string) error {
		t.Fatal("download must not run for cached pinned version")
		return nil
	}, nil)

	got, err := ensureXMLUIRepoIn(reposDir, "0.14.0")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// The #15 scenario: latest is newer than the cache and the network cannot
// deliver it. Startup must return the cached version immediately; the failed
// background refresh is observed via the hook and harmless.
func TestLatestServesCacheAndRefreshesInBackground(t *testing.T) {
	reposDir := t.TempDir()
	cached := makeValidCachedRepo(t, reposDir, "xmlui@0.14.0")

	refreshResult := make(chan error, 1)
	stubFetchers(t,
		func() (string, string, error) {
			return "xmlui@0.14.1", "https://example.invalid/xmlui@0.14.1.zip", nil
		},
		func(url, dest string) error {
			return errors.New("simulated slow-link failure")
		},
		func(tag string, err error) {
			if tag != "xmlui@0.14.1" {
				t.Errorf("background refresh tag = %q", tag)
			}
			refreshResult <- err
		},
	)

	got, err := ensureXMLUIRepoIn(reposDir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != cached {
		t.Fatalf("got %q, want cached %q", got, cached)
	}
	select {
	case bgErr := <-refreshResult:
		if bgErr == nil {
			t.Fatal("expected background refresh to fail")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("background refresh never completed")
	}
	if isRepoValid(filepath.Join(reposDir, "xmlui@0.14.1")) {
		t.Fatal("failed refresh must not install a repo")
	}
}

func TestLatestBackgroundRefreshSucceeds(t *testing.T) {
	reposDir := t.TempDir()
	cached := makeValidCachedRepo(t, reposDir, "xmlui@0.14.0")

	refreshResult := make(chan error, 1)
	stubFetchers(t,
		func() (string, string, error) {
			return "xmlui@0.14.1", "https://example.invalid/xmlui@0.14.1.zip", nil
		},
		func(url, dest string) error {
			stubArchive(t, dest)
			return nil
		},
		func(tag string, err error) { refreshResult <- err },
	)

	got, err := ensureXMLUIRepoIn(reposDir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != cached {
		t.Fatalf("startup must serve the cache, got %q", got)
	}
	select {
	case bgErr := <-refreshResult:
		if bgErr != nil {
			t.Fatalf("background refresh failed: %v", bgErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("background refresh never completed")
	}
	newDir := filepath.Join(reposDir, "xmlui@0.14.1")
	if !isRepoValid(newDir) {
		t.Fatal("background refresh did not install the new version")
	}
	// The per-version lock file survives: unlinking it after unlock is the
	// two-holders flock race (#15).
	if _, err := os.Stat(newDir + ".lock"); err != nil {
		t.Fatalf("lock file must remain in place: %v", err)
	}
}

// First run: no cache to serve, so the download blocks and its failure is the
// startup error (feedback (a) on the worklist item).
func TestLatestFirstRunBlocksAndFailsWithoutCache(t *testing.T) {
	reposDir := t.TempDir()
	stubFetchers(t,
		func() (string, string, error) {
			return "xmlui@0.14.1", "https://example.invalid/xmlui@0.14.1.zip", nil
		},
		func(url, dest string) error {
			return errors.New("simulated failure")
		},
		func(tag string, err error) {
			t.Error("no background refresh may run on first run")
		},
	)

	_, err := ensureXMLUIRepoIn(reposDir, "")
	if err == nil || !strings.Contains(err.Error(), "simulated failure") {
		t.Fatalf("expected download failure to surface, got %v", err)
	}
}

func TestLatestFirstRunSucceedsWithoutCache(t *testing.T) {
	reposDir := t.TempDir()
	stubFetchers(t,
		func() (string, string, error) {
			return "xmlui@0.14.1", "https://example.invalid/xmlui@0.14.1.zip", nil
		},
		func(url, dest string) error {
			stubArchive(t, dest)
			return nil
		},
		nil,
	)

	got, err := ensureXMLUIRepoIn(reposDir, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(reposDir, "xmlui@0.14.1") || !isRepoValid(got) {
		t.Fatalf("first-run install invalid: %q", got)
	}
}

func TestPinnedVersionFailureIsNotSubstituted(t *testing.T) {
	reposDir := t.TempDir()
	makeValidCachedRepo(t, reposDir, "xmlui@0.14.0")
	stubFetchers(t, nil,
		func(url, dest string) error {
			return errors.New("simulated failure")
		},
		func(tag string, err error) {
			t.Error("no background refresh may run for a pinned version")
		},
	)

	_, err := ensureXMLUIRepoIn(reposDir, "0.14.1")
	if err == nil || !strings.Contains(err.Error(), "simulated failure") {
		t.Fatalf("pinned-version failure must error, not substitute: %v", err)
	}
}
