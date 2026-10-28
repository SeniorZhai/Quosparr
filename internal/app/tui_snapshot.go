package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/runway"
)

const tuiSnapshotMaxAge = 45 * time.Second

type tuiSnapshot struct {
	GeneratedAt time.Time           `json:"generatedAt"`
	Floating    runway.FloatingView `json:"floating"`
	Language    l10n.Language       `json:"language"`
}

type tuiMirror struct {
	path   string
	mu     sync.Mutex
	view   runway.FloatingView
	lang   l10n.Language
	signal chan struct{}
	cancel context.CancelFunc
	done   chan struct{}
}

func tuiSnapshotPath(home string) string {
	return filepath.Join(platform.RunwayDir(home), "tui-snapshot.json")
}

func newTUIMirror(home string) *tuiMirror {
	ctx, cancel := context.WithCancel(context.Background())
	m := &tuiMirror{path: tuiSnapshotPath(home), signal: make(chan struct{}, 1), cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(m.done)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		failed := false
		for {
			select {
			case <-ctx.Done():
				return
			case <-m.signal:
			case <-ticker.C:
			}
			m.mu.Lock()
			view, lang := m.view, m.lang
			m.mu.Unlock()
			if len(view.Providers) == 0 {
				continue
			}
			err := writeTUIMirror(m.path, view, lang, time.Now())
			if err != nil && !failed {
				log.Println("TUI snapshot: unavailable")
			}
			failed = err != nil
		}
	}()
	return m
}

func (m *tuiMirror) publish(snap runway.Snapshot) {
	m.mu.Lock()
	m.view, m.lang = snap.Floating, snap.Language
	m.mu.Unlock()
	select {
	case m.signal <- struct{}{}:
	default:
	}
}

func (m *tuiMirror) close() { m.cancel(); <-m.done }

func writeTUIMirror(path string, view runway.FloatingView, lang l10n.Language, now time.Time) error {
	data, err := json.Marshal(tuiSnapshot{GeneratedAt: now, Floating: view, Language: lang})
	if err != nil {
		return err
	}
	if err := platform.EnsurePrivateDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	return platform.WriteFileAtomic(path, data, 0o600)
}

func readTUIMirror(path string, l l10n.Localizer, now time.Time) runway.FloatingView {
	message := xTUISnapshotFailed
	view, err := loadTUIMirror(path, now)
	if err == nil {
		return view.Floating
	}
	if errors.Is(err, os.ErrNotExist) {
		message = xTUIWaiting
	} else if errors.Is(err, errTUISnapshotStale) {
		message = xTUISnapshotStale
	}
	return runway.FloatingView{Providers: []runway.FloatingProvider{{Name: "Quosparr", Status: tx(l, message)}}}
}

var errTUISnapshotStale = errors.New("TUI snapshot is stale")

func loadTUIMirror(path string, now time.Time) (tuiSnapshot, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return tuiSnapshot{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return tuiSnapshot{}, errors.New("invalid TUI snapshot file")
	}
	if err := platform.VerifyOwnerOnly(path, 0o600); err != nil {
		return tuiSnapshot{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return tuiSnapshot{}, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return tuiSnapshot{}, errors.New("TUI snapshot changed")
	}
	data, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil || len(data) > 64<<10 {
		return tuiSnapshot{}, errors.New("TUI snapshot unreadable")
	}
	var snap tuiSnapshot
	if err := json.Unmarshal(data, &snap); err != nil || len(snap.Floating.Providers) == 0 {
		return tuiSnapshot{}, errors.New("invalid TUI snapshot")
	}
	if snap.GeneratedAt.IsZero() || now.Sub(snap.GeneratedAt) > tuiSnapshotMaxAge || snap.GeneratedAt.Sub(now) > 5*time.Second {
		return tuiSnapshot{}, errTUISnapshotStale
	}
	return snap, nil
}
