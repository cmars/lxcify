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
	"github.com/juju/errors"
	"gopkg.in/yaml.v1"
)

/*

# mounts define the devices on the host that the container will need access
# to.
mounts:

 # passthru mounts the same path on the host as in the container.

 - passthru: /dev/dri
   directory: true
 - passthru: /dev/snd
   directory: true
 - passthru: /tmp/.X11-unix
   directory: true
 - passthru: /dev/video0

  # Different paths on the host and container can be used as well.

 - host: /var/run/custom-video-source.sock
   container: /dev/video0

# Enable the container to share the host's pulseaudio socket for input and
# output.
share-pulse-audio: true

# Installation script for the application. Should be idempotent.
install-script: |
#!/bin/bash -x
export DEBIAN_FRONTEND=noninteractive
umount /tmp/.X11-unix
apt-get update
apt-get dist-upgrade -y
apt-get install -y --no-install-recommends wget ubuntu-artwork dmz-cursor-theme ca-certificates pulseaudio
wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb -O /tmp/chrome.deb
wget https://dl.google.com/linux/direct/google-talkplugin_current_amd64.deb -O /tmp/talk.deb
dpkg -i /tmp/chrome.deb /tmp/talk.deb
apt-get -f install -y --no-install-recommends
sudo -u ubuntu mkdir -p /home/ubuntu/.pulse/
sudo -u ubuntu tee /home/ubuntu/.pulse/client.conf <<EOF
disable-shm=yes
EOF

# Command which launches the application in the container.
launch-command: google-chrome --disable-setuid-sandbox $*

# Define a convenient desktop launcher to create on the host.
desktop-launcher:
  name: Google Chrome
  comment: LXCified Google Chrome Browser
  icon-path: /opt/google/chrome/product_logo_256.png
  categories:
   - Network
   - WebBrowser

*/

type appConfig struct {
	Mounts          []mountConfig          `yaml:"mounts,omitempty"`
	SharePulseAudio bool                   `yaml:"share-pulse-audio,omitempty"`
	InstallScript   string                 `yaml:"install-script"`
	LaunchCommand   string                 `yaml:"launch-command"`
	DesktopLauncher *desktopLauncherConfig `yaml:"desktop-launcher,omitempty"`
}

type mountConfig struct {
	Passthru  string `yaml:"passthru,omitempty"`
	Host      string `yaml:"host,omitempty"`
	Container string `yaml:"container,omitempty"`
	IsDir     bool   `yaml:"directory,omitempty"`
}

type desktopLauncherConfig struct {
	Name       string   `yaml:"name"`
	Comment    string   `yaml:"comment,omitempty"`
	IconPath   string   `yaml:"icon-path"`
	Categories []string `yaml:"categories,omitempty"`
}

func (c *appConfig) app() (*App, error) {
	if c.InstallScript == "" {
		return nil, errors.New("missing install-script")
	}
	if c.LaunchCommand == "" {
		return nil, errors.New("missing launch-command")
	}

	app := &App{
		InstallScript:   c.InstallScript,
		LaunchCommand:   c.LaunchCommand,
		SharePulseAudio: c.SharePulseAudio,
	}

	for _, mountConfig := range c.Mounts {
		mount, err := mountConfig.mount()
		if err != nil {
			return nil, errors.Trace(err)
		}
		app.Mounts = append(app.Mounts, mount)
	}
	app.DesktopLauncher = c.DesktopLauncher.desktopLauncher()
	return app, nil
}

var errMountConfigInvalid = errors.New("{passthru} is mutually exclusive with {host,container}")

func (c *mountConfig) mount() (Mount, error) {
	fail := Mount{}
	if c.Passthru != "" {
		if c.Host != "" || c.Container != "" {
			return fail, errors.Trace(errMountConfigInvalid)
		}
		return PassthruMount(c.Passthru, c.IsDir), nil
	} else if c.Host != "" && c.Container != "" {
		return Mount{
			Host:      c.Host,
			Container: c.Container[1:],
			IsDir:     c.IsDir,
		}, nil
	}
	return fail, errors.New("missing required fields: {passthru} or {host,container}")
}

func (c *desktopLauncherConfig) desktopLauncher() *DesktopLauncher {
	if c == nil {
		return nil
	}
	return &DesktopLauncher{
		Name:       c.Name,
		Comment:    c.Comment,
		IconPath:   c.IconPath,
		Categories: c.Categories,
	}
}

func ParseApp(in []byte) (*App, error) {
	var appConfig appConfig
	err := yaml.Unmarshal(in, &appConfig)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return appConfig.app()
}
