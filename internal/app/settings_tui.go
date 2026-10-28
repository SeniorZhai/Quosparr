package app

import (
	"errors"
	"os"

	"github.com/egoist/mygo/ui"
	"github.com/licoy/quosparr/internal/platform"
	"github.com/licoy/quosparr/internal/tuiinstall"
)

func installBundledTUI() (string, error) {
	home, err := platform.Home()
	if err != nil {
		return "", tuiinstall.ErrInstall
	}
	executable, err := os.Executable()
	if err != nil {
		return "", tuiinstall.ErrSource
	}
	return tuiinstall.Install(home, tuiinstall.BundledPath(executable))
}

func (u *settingsUI) tuiGroup(c *ui.Context) {
	l := u.L()
	settingsGroup(c, "", "", nil, func(row rowFunc) {
		row(tx(l, xTUICommand), tx(l, xTUICommandDescription), func() {
			if u.tuiInstallTask.Running {
				ui.Spinner(c).Size(14, 14)
			}
			if PillButton(c, nil, tx(l, xTUIInstall), u.tuiInstallTask.Running).Clicked() {
				var path string
				runTask(&u.tuiInstallTask, u.post, func() (err error) {
					path, err = u.opts.Actions.InstallTUI()
					return err
				}, func(err error) {
					message := tx(l, xTUIInstalled, path)
					if err != nil {
						key := xTUIInstallFailed
						if errors.Is(err, tuiinstall.ErrSource) {
							key = xTUIInstallSource
						}
						if errors.Is(err, tuiinstall.ErrConflict) {
							key = xTUIInstallConflict
						}
						message = tx(l, key)
					}
					u.showNotice(tx(l, xTUIInstall), message)
				})
			}
		})
	})
}
