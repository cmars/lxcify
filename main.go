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

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"text/template"
	"time"

	"github.com/juju/errors"
	"github.com/juju/loggo"
	"gopkg.in/lxc/go-lxc.v1"
)

var logger = loggo.GetLogger("nfsz")

func init() {
	logger.SetLogLevel(loggo.DEBUG)
}

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

func (c *Container) setLxcConfig(items []lxcConfigItem) error {
	for _, item := range items {
		err := c.SetConfigItem(item.key, item.value)
		if err != nil {
			return errors.Annotatef(err, "key=%q value=%q", item.key, item.value)
		}
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

type App struct {
	InstallScript string
	LaunchCommand string
	Launcher      DesktopEntry
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
		die(err)
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
	err = c.installDesktopLauncher(app)
	if err != nil {
		return errors.Trace(err)
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
		fmt.Sprintf("%s.desktop", app.Launcher.Name)),
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
		DesktopEntry
		ConfigPath string
		LxcName    string
	}{
		DesktopEntry: app.Launcher,
		ConfigPath:   c.ConfigPath(),
		LxcName:      c.Name(),
	})
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}

type DesktopEntry struct {
	Name       string
	Comment    string
	IconPath   string
	Categories []string
}

func die(err error) {
	if err != nil {
		logger.Errorf("%v", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func main() {
	c, err := NewContainer("rubik")
	if err != nil {
		die(err)
	}
	err = c.Create()
	if err != nil {
		die(err)
	}

	err = c.Start()
	if err != nil {
		die(err)
	}

	app := &App{
		InstallScript: `#!/bin/bash -x
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
`,
		LaunchCommand: "google-chrome --disable-setuid-sandbox $*",
		Launcher: DesktopEntry{
			Name:       "Google Chrome",
			Comment:    "LXC",
			IconPath:   "/opt/google/chrome/product_logo_256.png",
			Categories: []string{"Network", "WebBrowser"},
		},
	}
	err = c.Install(app)
	if err != nil {
		die(err)
	}

	err = c.Stop()
	if err != nil {
		die(err)
	}

	die(nil)
}
