package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
)

func TestTUIHeldLockMirrorsWithoutStartingProviders(t *testing.T) {
	home := t.TempDir()
	lock, err := platform.AcquireLock(platform.LockPath(home))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Release()
	want := runway.FloatingView{Providers: []runway.FloatingProvider{{Provider: store.ProviderCodex, Name: "Codex", Status: "Wait"}}}
	if err := writeTUIMirror(tuiSnapshotPath(home), want, l10n.SimplifiedChinese, time.Now()); err != nil {
		t.Fatal(err)
	}
	s, err := openTUISession(context.Background(), home, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	if s.refresh != nil || s.l.Language() != l10n.SimplifiedChinese || !reflect.DeepEqual(s.snapshot(), want) {
		t.Fatalf("held lock did not select the read-only mirror: %+v", s.snapshot())
	}
	entries, err := os.ReadDir(platform.RunwayDir(home))
	if err != nil || len(entries) != 2 {
		t.Fatalf("mirror created unexpected provider state: %v, %v", entries, err)
	}
	if state, err := platform.ProbeLock(platform.LockPath(home)); err != nil || state != platform.LockHeld {
		t.Fatalf("mirror released its source's lock: %v, %v", state, err)
	}
}

func TestTUIRejectsLegacyInstanceBeforeLoadingProviders(t *testing.T) {
	home := t.TempDir()
	legacy, err := platform.AcquireLock(filepath.Join(home, ".codex-runway", "codex-runway.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Release()
	if s, err := openTUISession(context.Background(), home, "test"); err == nil || s != nil {
		if s != nil {
			s.close()
		}
		t.Fatal("legacy instance allowed a second provider runtime")
	}
	if state, err := platform.ProbeLock(platform.LockPath(home)); err != nil || state != platform.LockFree {
		t.Fatalf("failed startup retained the current lock: %v, %v", state, err)
	}
}

func TestTUIMirrorRejectsStaleCorruptAndLinkedData(t *testing.T) {
	path := tuiSnapshotPath(t.TempDir())
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	view := runway.FloatingView{Providers: []runway.FloatingProvider{{Provider: store.ProviderClaude, Name: "Claude", Meters: []runway.FloatingMeter{{RemainingPercent: 72, Reset: "2h"}}}}}
	if err := writeTUIMirror(path, view, l10n.English, now); err != nil {
		t.Fatal(err)
	}
	if err := platform.VerifyOwnerOnly(path, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadTUIMirror(path, now)
	if err != nil || !reflect.DeepEqual(got.Floating, view) {
		t.Fatalf("snapshot round trip = %+v, %v", got, err)
	}
	l := l10n.New(l10n.English)
	stale := readTUIMirror(path, l, now.Add(tuiSnapshotMaxAge+time.Second))
	if len(stale.Providers) != 1 || stale.Providers[0].Status != tx(l, xTUISnapshotStale) || len(stale.Providers[0].Meters) != 0 {
		t.Fatalf("stale quotas shown as live: %+v", stale)
	}
	if _, err := loadTUIMirror(path, now.Add(-time.Minute)); !errors.Is(err, errTUISnapshotStale) {
		t.Fatal("future snapshot accepted")
	}
	if err := platform.WriteFileAtomic(path, []byte(`{"floating":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readTUIMirror(path, l, now); got.Providers[0].Status != tx(l, xTUISnapshotFailed) {
		t.Fatalf("corrupt snapshot = %+v", got)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := readTUIMirror(path, l, now); got.Providers[0].Status != tx(l, xTUIWaiting) {
		t.Fatalf("missing snapshot = %+v", got)
	}
	target := filepath.Join(filepath.Dir(path), "other.json")
	if err := writeTUIMirror(target, view, l10n.English, now); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skip("symlinks unavailable")
	}
	if _, err := loadTUIMirror(path, now); err == nil {
		t.Fatal("linked snapshot accepted")
	}
}

func TestTUIMirrorPublishesAndStopsWithRuntime(t *testing.T) {
	home := t.TempDir()
	m := newTUIMirror(home)
	t.Cleanup(m.close)
	view := runway.FloatingView{Providers: []runway.FloatingProvider{{Name: "Codex", Status: "Wait"}}}
	m.publish(runway.Snapshot{Floating: view})
	deadline := time.Now().Add(2 * time.Second)
	for {
		if got, err := loadTUIMirror(m.path, time.Now()); err == nil && reflect.DeepEqual(got.Floating, view) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("mirror did not publish")
		}
		time.Sleep(time.Millisecond)
	}
	m.close()
	before, err := os.ReadFile(m.path)
	if err != nil {
		t.Fatal(err)
	}
	m.publish(runway.Snapshot{Floating: runway.FloatingView{Providers: []runway.FloatingProvider{{Name: "Changed"}}}})
	after, err := os.ReadFile(m.path)
	if err != nil || string(before) != string(after) {
		t.Fatal("stopped runtime continued publishing")
	}
}
