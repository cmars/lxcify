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
	"fmt"
	"time"

	"github.com/juju/errors"
	"github.com/juju/loggo"
	"gopkg.in/lxc/go-lxc.v1"
)

var logger = loggo.GetLogger("lxcify")

type Mount struct {
	Host, Container string
	IsDir           bool
}

func (m Mount) lxcConfigItem() lxcConfigItem {
	create := "file"
	if m.IsDir {
		create = "dir"
	}
	return lxcConfigItem{"lxc.mount.entry", fmt.Sprintf("%s %s none bind,optional,create=%s",
		m.Host, m.Container, create)}
}

func PassthruMount(path string, isDir bool) Mount {
	return Mount{
		Host:      path,
		Container: path[1:],
		IsDir:     isDir,
	}
}

var (
	MountDRI    = PassthruMount("/dev/dri", true)
	MountSnd    = PassthruMount("/dev/snd", true)
	MountX11    = PassthruMount("/tmp/.X11-unix", true)
	MountVideo0 = PassthruMount("/dev/video0", false)

	defaultMounts = []Mount{
		MountDRI, MountSnd, MountX11, MountVideo0,
	}
)

type lxcConfigItem struct {
	key, value string
}

var defaultLxcConfig = []lxcConfigItem{
	{"lxc.aa_profile", "lxc-container-default"},
}

type Container struct {
	*lxc.Container

	lxcpath  string
	template string
	distro   string
	release  string
	arch     string

	mounts     []Mount
	pulseAudio bool
}

type Option func(*Container) error

var defaultOptions = []Option{
	ConfigPath(lxc.DefaultConfigPath()),
	Template("ubuntu"),
	Target("ubuntu", "trusty", "amd64"),
	Mounts(defaultMounts...),
	PulseAudio(true),
}

func ConfigPath(lxcpath string) Option {
	return func(c *Container) error {
		c.lxcpath = lxcpath
		return nil
	}
}

func Template(template string) Option {
	return func(c *Container) error {
		c.template = template
		return nil
	}
}

func Target(distro, release, arch string) Option {
	return func(c *Container) error {
		c.distro, c.release, c.arch = distro, release, arch
		return nil
	}
}

func Mounts(mounts ...Mount) Option {
	return func(c *Container) error {
		c.mounts = append(c.mounts, mounts...)
		return nil
	}
}

func PulseAudio(enable bool) Option {
	return func(c *Container) error {
		c.pulseAudio = enable
		return nil
	}
}

func NewContainer(name string, options ...Option) (*Container, error) {
	c := &Container{}

	if len(options) == 0 {
		options = defaultOptions
	}

	var err error
	for _, option := range options {
		err = option(c)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	c.Container, err = lxc.NewContainer(name, c.lxcpath)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return c, nil
}

func (c *Container) setLxcConfig(items []lxcConfigItem) error {
	for _, item := range items {
		err := c.SetConfigItem(item.key, item.value)
		if err != nil {
			return errors.Annotatef(err, "key=%q value=%q", item.key, item.value)
		}
	}
	return nil
}

func (c *Container) Start() error {
	err := c.Container.Start()
	if err != nil {
		return errors.Trace(err)
	}
	ok := c.Wait(lxc.RUNNING, 10)
	if !ok {
		return errors.New("timeout waiting for container to start")
	}

	// TODO: poll for container network availability, sleeps suck
	time.Sleep(5 * time.Second)
	return nil
}
