package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/licoy/quosparr/internal/claude"
	"github.com/licoy/quosparr/internal/codex"
	"github.com/licoy/quosparr/internal/codexapi"
	"github.com/licoy/quosparr/internal/grok"
	"github.com/licoy/quosparr/internal/l10n"
	"github.com/licoy/quosparr/internal/livespeed"
	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/runway"
	"github.com/licoy/quosparr/internal/store"
	"github.com/licoy/quosparr/internal/tui"
	"github.com/licoy/quosparr/internal/usage"
)

type tuiSession struct {
	l        l10n.Localizer
	snapshot func() runway.FloatingView
	refresh  func()
	close    func()
}

// RunTUI reuses a running instance's snapshot or owns the normal account lock.
func RunTUI(ctx context.Context, in, out *os.File, version string) error {
	if err := tui.CheckTerminal(in, out); err != nil {
		return errors.New(l10n.ForPreference("system", l10n.SystemPreferredLanguages()).TUI("terminalRequired"))
	}
	home, err := platform.Home()
	if err != nil {
		return errors.New(tx(l10n.ForPreference("system", l10n.SystemPreferredLanguages()), xTUIStartFailed))
	}
	s, err := openTUISession(ctx, home, version)
	if err != nil {
		return err
	}
	defer s.close()
	return tui.Run(ctx, in, out, s.l, s.snapshot, s.refresh)
}

func openTUISession(ctx context.Context, home, version string) (*tuiSession, error) {
	l := l10n.ForPreference("system", l10n.SystemPreferredLanguages())
	lock, err := platform.AcquireLock(platform.LockPath(home))
	if errors.Is(err, platform.ErrLocked) {
		if snap, err := loadTUIMirror(tuiSnapshotPath(home), time.Now()); err == nil && snap.Language != "" {
			l = l10n.ForPreference(string(snap.Language), nil)
		}
		return &tuiSession{l: l, snapshot: func() runway.FloatingView {
			return readTUIMirror(tuiSnapshotPath(home), l, time.Now())
		}, close: func() {}}, nil
	}
	if err != nil {
		return nil, errors.New(tx(l, xTUIStartFailed))
	}
	legacy, err := platform.AcquireLegacyLock(home, platform.AcquireLock)
	if err != nil {
		_ = lock.Release()
		if errors.Is(err, platform.ErrLocked) {
			return nil, errors.New(tx(l, xTUILegacyRunning))
		}
		return nil, errors.New(tx(l, xTUIStartFailed))
	}
	release := func() { _ = legacy.Release(); _ = lock.Release() }
	st, dev, err := OpenStore()
	if err != nil {
		release()
		return nil, errors.New(tx(l, xSettingsUnreadable))
	}
	creds := NewPlatformCredentials()
	settings := NewSettings(st, System{}, creds)
	if err := settings.Load(); err != nil {
		release()
		return nil, errors.New(tx(l, xSettingsUnreadable))
	}
	l = settings.L()
	if err := settings.LoadProxy(); err != nil {
		release()
		return nil, errors.New(tx(l, xProxyBlocked))
	}
	InstallStartupProxy(settings, creds)
	if !settings.BackgroundAllowed() {
		release()
		return nil, errors.New(tx(l, xProxyBlocked))
	}
	if err := ctx.Err(); err != nil {
		release()
		return nil, err
	}
	prefs := settings.Prefs()
	prefs.FloatingWindow.Enabled = true
	mirror := newTUIMirror(home)
	svc, monitor := newTUIService(ctx, home, version, prefs, dev, mirror.publish)
	svc.Start(ctx)
	return &tuiSession{l: l, snapshot: func() runway.FloatingView { return svc.Snapshot().Floating },
		refresh: func() { svc.Refresh(usage.RefreshForce) },
		close: func() {
			monitor.Close()
			svc.Close()
			mirror.close()
			release()
		},
	}, nil
}

func newTUIService(ctx context.Context, home, version string, prefs store.Preferences, dev bool, publish func(runway.Snapshot)) (*runway.Service, *livespeed.Monitor) {
	langs := l10n.SystemPreferredLanguages()
	root := platform.RunwayDir(home)
	codexStore := codex.NewStore(platform.CodexDir(home), root)
	gstore := grok.NewStore(grok.ResolveOfficialHome(os.Getenv("GROK_HOME"), home), root)
	gmod := grok.NewModule(gstore)
	gmod.AppVersion, gmod.Billing = version, (&grok.BillingClient{}).Fetch
	_, _ = gmod.Load(ctx)
	gbridge := newGrokBridge(gmod, prefs, langs)
	gbridge.reload(false)
	cstore := claude.NewStore(claude.ResolveConfigDir(os.Getenv("CLAUDE_CONFIG_DIR"), home), root, home, os.Getenv("CLAUDE_CONFIG_DIR"), "")
	if dev {
		cstore.Official = claude.FileStore{Path: claude.CredentialsFilePath(cstore.ConfigDir())}
	}
	cmod := claude.NewModule(cstore)
	cmod.Usage = &claude.UsageClient{Version: claudeCLIVersion(cstore.ConfigDir())}
	cmod.Tokens = claude.RefreshClient{}
	cbridge := newClaudeBridge(cmod, prefs, langs)
	cbridge.reload(false)
	monitor := livespeed.Start(livespeed.Homes{
		Codex: platform.CodexDir(home), Grok: gstore.OfficialHome(), ClaudeProjects: claude.ProjectsDirs(cstore.ConfigDir(), home),
	})
	svc := runway.New(runway.Options{
		Sources: runway.NewLiveSources(runway.LiveConfig{Store: codexStore, CodexHome: platform.CodexDir(home),
			Quota: codexapi.QuotaClient{}, Refresher: codexapi.TokenRefresher{}}),
		Preferences: prefs, Grok: gbridge, Claude: cbridge, RemoteQuotaDir: filepath.Join(home, store.DataDirName),
		SystemLanguages: langs, OnSnapshot: publish,
		LiveSpeed: func() []runway.LivePart {
			parts := monitor.Current()
			out := make([]runway.LivePart, 0, len(parts))
			for _, part := range parts {
				out = append(out, runway.LivePart{Provider: store.Provider(part.Source), Text: part.Text, Pending: part.Pending})
			}
			return out
		},
	})
	return svc, monitor
}
