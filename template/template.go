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

package template

import (
	"runtime"

	"github.com/juju/errors"
	"gopkg.in/lxc/go-lxc.v1"
	"gopkg.in/yaml.v1"

	"github.com/cmars/lxcify"
)

type Template struct {
	ContainerInfo   container        `yaml:"container"`
	Mounts          []mount          `yaml:"mounts,omitempty"`
	SharePulseAudio bool             `yaml:"share-pulse-audio,omitempty"`
	InstallScript   string           `yaml:"install-script"`
	LaunchCommand   string           `yaml:"launch-command"`
	DesktopLauncher *desktopLauncher `yaml:"desktop-launcher,omitempty"`
}

type container struct {
	Template string `yaml:"template"`
	Distro   string `yaml:"distro"`
	Release  string `yaml:"release"`
	Arch     string `yaml:"arch"`
}

func defaultArch() string {
	a := runtime.GOARCH
	switch a {
	case "386":
		return "i386"
	case "amd64":
		return a
	case "arm":
		return "arm"
	default:
		return a
	}
}

var defaultContainerConfig = container{
	Template: "ubuntu",
	Distro:   "ubuntu",
	Release:  "trusty",
	Arch:     defaultArch(),
}

type mount struct {
	Passthru  string `yaml:"passthru,omitempty"`
	Host      string `yaml:"host,omitempty"`
	Container string `yaml:"container,omitempty"`
	IsDir     bool   `yaml:"directory,omitempty"`
}

type desktopLauncher struct {
	Name     string `yaml:"name"`
	Comment  string `yaml:"comment,omitempty"`
	IconPath string `yaml:"icon-path"`
}

func (t *Template) Container(name string) (*lxcify.Container, error) {
	mounts, err := t.mounts()
	if err != nil {
		return nil, errors.Trace(err)
	}

	options := []lxcify.Option{
		lxcify.ConfigPath(lxc.DefaultConfigPath()),
		lxcify.Template(t.ContainerInfo.Template),
		lxcify.Target(t.ContainerInfo.Distro, t.ContainerInfo.Release, t.ContainerInfo.Arch),
		lxcify.Mounts(mounts...),
	}
	if t.SharePulseAudio {
		options = append(options, lxcify.PulseAudio(true))
	}
	return lxcify.NewContainer(name, options...)
}

func (t *Template) App() (*lxcify.App, error) {
	if t.InstallScript == "" {
		return nil, errors.New("missing install-script")
	}
	if t.LaunchCommand == "" {
		return nil, errors.New("missing launch-command")
	}

	app := &lxcify.App{
		InstallScript: t.InstallScript,
		LaunchCommand: t.LaunchCommand,
	}
	app.DesktopLauncher = t.DesktopLauncher.desktopLauncher()
	return app, nil
}

func (t *Template) mounts() ([]lxcify.Mount, error) {
	var mounts []lxcify.Mount
	for _, mountConfig := range t.Mounts {
		mount, err := mountConfig.mount()
		if err != nil {
			return nil, errors.Trace(err)
		}
		mounts = append(mounts, mount)
	}
	return mounts, nil
}

var errMountConfigInvalid = errors.New("{passthru} is mutually exclusive with {host,container}")

func (m *mount) mount() (lxcify.Mount, error) {
	fail := lxcify.Mount{}
	if m.Passthru != "" {
		if m.Host != "" || m.Container != "" {
			return fail, errors.Trace(errMountConfigInvalid)
		}
		return lxcify.PassthruMount(m.Passthru, m.IsDir), nil
	} else if m.Host != "" && m.Container != "" {
		return lxcify.Mount{
			Host:      m.Host,
			Container: m.Container[1:],
			IsDir:     m.IsDir,
		}, nil
	}
	return fail, errors.New("missing required fields: {passthru} or {host,container}")
}

func (dl *desktopLauncher) desktopLauncher() *lxcify.DesktopLauncher {
	if dl == nil {
		return nil
	}
	return &lxcify.DesktopLauncher{
		Name:     dl.Name,
		Comment:  dl.Comment,
		IconPath: dl.IconPath,
	}
}

func Parse(in []byte) (*Template, error) {
	var template Template
	err := yaml.Unmarshal(in, &template)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &template, nil
}
