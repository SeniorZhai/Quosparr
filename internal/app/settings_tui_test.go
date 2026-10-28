package app

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/tuiinstall"
)

func TestSettingsInstallsTUIOnWorkerAndReportsResult(t *testing.T) {
	for _, failure := range []error{nil, tuiinstall.ErrConflict, tuiinstall.ErrSource, errors.New("private detail")} {
		e := newEnv(t, nil)
		u, tt := openSettingsView(t, e, tabAdvanced)
		tt.SetPreferences(ui.Preferences{ReduceMotion: true})
		release := make(chan struct{})
		u.opts.Actions.InstallTUI = func() (string, error) { <-release; return "/fixture/quosparr", failure }
		tt.Frame()
		focusSettingsAction(t, tt, tx(e.s.L(), xTUIInstall))
		start := time.Now()
		tt.Key(0, ui.KeySpace)
		if !u.tuiInstallTask.Running || time.Since(start) > latencyBudget(100*time.Millisecond) {
			close(release)
			t.Fatal("installation blocked the UI")
		}
		close(release)
		awaitSettingsTask(t, e, &u.tuiInstallTask)
		want := tx(e.s.L(), xTUIInstalled, "/fixture/quosparr")
		switch failure {
		case nil:
		case tuiinstall.ErrConflict:
			want = tx(e.s.L(), xTUIInstallConflict)
		case tuiinstall.ErrSource:
			want = tx(e.s.L(), xTUIInstallSource)
		default:
			want = tx(e.s.L(), xTUIInstallFailed)
		}
		if !u.noticeOpen || u.notice != want {
			t.Fatalf("notice = %q; want %q", u.notice, want)
		}
	}
}
