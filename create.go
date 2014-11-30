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
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/juju/errors"
	"gopkg.in/lxc/go-lxc.v1"
)

func (c *Container) Create() error {
	c.SetVerbosity(lxc.Verbose)

	err := c.CreateAsUser(c.distro, c.release, c.arch)
	if err != nil {
		return errors.Trace(err)
	}

	err = c.setupUserPassthru()
	if err != nil {
		return errors.Trace(err)
	}

	var configItems []lxcConfigItem
	configItems = append(configItems, defaultLxcConfig...)
	for _, mount := range c.mounts {
		configItems = append(configItems, mount.lxcConfigItem())
	}
	err = c.setLxcConfig(configItems)
	if err != nil {
		return errors.Trace(err)
	}

	if c.pulseAudio {
		err = c.setupPulseAudio()
		if err != nil {
			return errors.Trace(err)
		}
	}

	err = c.SaveConfigFile(c.ConfigFileName())
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

// clearIdMap removes lxc.id_map entries from the container config, using a
// special workaround. go-lxc can't remove 'lxc.id_map' entries with
// lxc.ClearConfigItem.
func clearIdMap(c *lxc.Container) error {
	contents, err := ioutil.ReadFile(c.ConfigFileName())
	if err != nil {
		return errors.Trace(err)
	}
	c.ClearConfig()
	lines := bytes.Split(contents, []byte{'\n'})
	err = func() error {
		f, err := os.OpenFile(c.ConfigFileName(), os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return errors.Trace(err)
		}
		defer f.Close()
		for i := range lines {
			if !bytes.HasPrefix(lines[i], []byte("lxc.id_map")) {
				_, err = fmt.Fprintln(f, string(lines[i]))
				if err != nil {
					return errors.Trace(err)
				}
			}
		}
		return nil
	}()
	if err != nil {
		return errors.Trace(err)
	}
	return c.LoadConfigFile(c.ConfigFileName())
}

func (c *Container) setupUserPassthru() error {
	err := clearIdMap(c.Container)
	if err != nil {
		return errors.Trace(err)
	}

	startLxcId := 100000
	nLxcIds := 65535
	uid, gid := os.Getuid(), os.Getgid()
	items := []lxcConfigItem{
		{"lxc.id_map", fmt.Sprintf("u 0 %d %d", startLxcId, uid)},
		{"lxc.id_map", fmt.Sprintf("g 0 %d %d", startLxcId, gid)},
		{"lxc.id_map", fmt.Sprintf("u %d %d 1", uid, uid)},
		{"lxc.id_map", fmt.Sprintf("g %d %d 1", gid, gid)},
		{"lxc.id_map", fmt.Sprintf("u %d %d %d", uid+1, startLxcId+uid+1, nLxcIds-uid)},
		{"lxc.id_map", fmt.Sprintf("g %d %d %d", gid+1, startLxcId+gid+1, nLxcIds-gid)},
	}
	err = errors.Trace(c.setLxcConfig(items))
	if err != nil {
		return errors.Trace(err)
	}

	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(
		"sudo chown -R %d:%d %s", uid, gid,
		path.Join(c.ConfigPath(), c.Name(), "rootfs", "home", "ubuntu")))
	return errors.Trace(cmd.Run())
}

const setupPulseScript = `#!/bin/sh
PULSE_PATH=$LXC_ROOTFS_PATH/home/ubuntu/.pulse_socket

if [ ! -e "$PULSE_PATH" ] || [ -z "$(lsof -n $PULSE_PATH 2>&1)" ]; then
    pactl load-module module-native-protocol-unix auth-anonymous=1 \
        socket=$PULSE_PATH
fi
`

func (c *Container) setupPulseAudio() error {
	f, err := os.OpenFile(path.Join(c.ConfigPath(), c.Name(), "setup-pulse.sh"),
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0700)
	if err != nil {
		return errors.Trace(err)
	}
	defer f.Close()
	_, err = fmt.Fprint(f, setupPulseScript)
	if err != nil {
		return errors.Trace(err)
	}
	return errors.Trace(c.SetConfigItem("lxc.hook.pre-start", f.Name()))
}
