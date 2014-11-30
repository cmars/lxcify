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
	"log"
	"os"

	"github.com/cmars/lxcify"
)

func die(err error) {
	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(0)
}

func main() {
	c, err := lxcify.NewContainer("rubik")
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

	app := &lxcify.App{
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
		DesktopLauncher: &lxcify.DesktopLauncher{
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
