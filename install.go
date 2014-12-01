/* Copyright (c) 2014 Casey Marshall

   This file is part of lxcify.

   lxcify is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, version 3.

   Foobar is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with lxcify. If not, see <http://www.gnu.org/licenses/>.
*/

package lxcify

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"text/template"

	"github.com/juju/errors"
)

type App struct {
	InstallScript   string
	LaunchCommand   string
	DesktopLauncher *DesktopLauncher
}

type DesktopLauncher struct {
	Name       string
	Comment    string
	IconPath   string
	Categories []string
}

const launchScript = `#!/bin/sh
CONTAINER={{.Name}}
CMD_LINE="{{.LaunchCommand}}"

STARTED=false

if ! lxc-wait -n $CONTAINER -s RUNNING -t 0; then
    lxc-start -n $CONTAINER -d
    lxc-wait -n $CONTAINER -s RUNNING
    STARTED=true
fi

PULSE_SOCKET=/home/ubuntu/.pulse_socket

lxc-attach --clear-env -n $CONTAINER -- sudo -u ubuntu -i \
    env DISPLAY=$DISPLAY PULSE_SERVER=$PULSE_SOCKET $CMD_LINE

if [ "$STARTED" = "true" ]; then
    lxc-stop -n $CONTAINER -t 10
fi
`

const desktopLauncher = `[Desktop Entry]
Version=1.0
Name={{.Name}}
Comment={{.Comment}}
Exec={{.ConfigPath}}/{{.LxcName}}/launch.sh %U
Icon={{.ConfigPath}}/{{.LxcName}}/rootfs{{.IconPath}}
Type=Application
{{if .Categories}}Categories={{range .Categories}}{{.}}{{end}}{{end}}
`

func (c *Container) Install(app *App) error {
	if !c.Running() {
		err := c.Start()
		if err != nil {
			return errors.Trace(err)
		}
	}

	// Execute install script in container
	r, w, err := os.Pipe()
	if err != nil {
		return errors.Trace(err)
	}
	defer r.Close()
	go func() {
		_, err := io.Copy(w, bytes.NewBufferString(app.InstallScript))
		if err != nil {
			logger.Errorf("%v", errors.Trace(err))
			return
		}
		defer w.Close()
	}()
	err = c.RunCommand(r.Fd(), os.Stdout.Fd(), os.Stderr.Fd(), "/bin/sh", "-c", "cat >/tmp/install.sh")
	if err != nil {
		return errors.Trace(err)
	}
	err = c.RunCommand(os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd(), "/bin/bash", "/tmp/install.sh")
	if err != nil {
		return errors.Trace(err)
	}

	// Create launcher script
	err = c.installLauncherScript(app)
	if err != nil {
		return errors.Trace(err)
	}

	// Create desktop launcher
	if app.DesktopLauncher != nil {
		err = c.installDesktopLauncher(app)
		if err != nil {
			return errors.Trace(err)
		}
	}

	return nil
}

func (c *Container) installLauncherScript(app *App) error {
	f, err := os.OpenFile(path.Join(c.ConfigPath(), c.Name(), "launch.sh"),
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0700)
	if err != nil {
		return errors.Trace(err)
	}
	defer f.Close()
	t, err := template.New("launch").Parse(launchScript)
	if err != nil {
		return errors.Trace(err)
	}
	err = t.Execute(f, struct {
		Name, LaunchCommand string
	}{c.Name(), app.LaunchCommand})
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (c *Container) installDesktopLauncher(app *App) error {
	f, err := os.OpenFile(path.Join(os.Getenv("HOME"), ".local", "share", "applications",
		fmt.Sprintf("%s.desktop", app.DesktopLauncher.Name)),
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return errors.Trace(err)
	}
	defer f.Close()
	t, err := template.New("launch").Parse(desktopLauncher)
	if err != nil {
		return errors.Trace(err)
	}
	err = t.Execute(f, struct {
		*DesktopLauncher
		ConfigPath string
		LxcName    string
	}{
		DesktopLauncher: app.DesktopLauncher,
		ConfigPath:      c.ConfigPath(),
		LxcName:         c.Name(),
	})
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
