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

package lxcify_test

import (
	gc "launchpad.net/gocheck"
	stdtesting "testing"

	"github.com/cmars/lxcify"
)

func Test(t *stdtesting.T) {
	gc.TestingT(t)
}

type ConfigSuite struct{}

var _ = gc.Suite(&ConfigSuite{})

var testYaml = `
mounts:
 - passthru: /dev/dri
   directory: true
 - host: /dev/video1
   container: /dev/video0
share-pulse-audio: true
install-script: |
   apt-get update -y
   apt-get dist-upgrade -y
   apt-get install -y --no-install-recommends beef

launch-command: /bin/beef
desktop-launcher:
  name: beef
  comment: beefy beef
  icon-path: /usr/share/pixmaps/big-juicy-ribeye.png
  categories:
   - mmm
   - beef
`

func (*ConfigSuite) TestConfigContent(c *gc.C) {
	app, err := lxcify.ParseApp([]byte(testYaml))
	c.Assert(err, gc.IsNil)
	c.Assert(app, gc.NotNil)

	c.Assert(app.Mounts, gc.HasLen, 2)
	c.Assert(app.Mounts[0], gc.DeepEquals,
		lxcify.Mount{
			Host:      "/dev/dri",
			Container: "dev/dri",
			IsDir:     true,
		})
	c.Assert(app.Mounts[1], gc.DeepEquals,
		lxcify.Mount{
			Host:      "/dev/video1",
			Container: "dev/video0",
			IsDir:     false,
		})
	c.Assert(app.SharePulseAudio, gc.Equals, true)
	c.Assert(app.InstallScript, gc.Matches, "(?m).*apt-get update.*")
	c.Assert(app.LaunchCommand, gc.Equals, "/bin/beef")
	c.Assert(app.DesktopLauncher, gc.NotNil)
	c.Assert(app.DesktopLauncher.Name, gc.Equals, "beef")
	c.Assert(app.DesktopLauncher.Comment, gc.Equals, "beefy beef")
	c.Assert(app.DesktopLauncher.IconPath, gc.Matches, ".*ribeye[.]png$")
	c.Assert(app.DesktopLauncher.Categories, gc.HasLen, 2)
	c.Assert(app.DesktopLauncher.Categories[0], gc.Equals, "mmm")
	c.Assert(app.DesktopLauncher.Categories[1], gc.Equals, "beef")
}

func (*ConfigSuite) TestConfigParsing(c *gc.C) {
	testCases := []struct {
		yaml       string
		errPattern string
	}{{
		yaml: testYaml,
	}, {
		yaml:       "}{",
		errPattern: "YAML error:.*",
	}, {
		yaml:       "nope:nope:nope",
		errPattern: "missing install-script",
	}}

	for i, testCase := range testCases {
		c.Log("test#", i)
		app, err := lxcify.ParseApp([]byte(testCase.yaml))
		if testCase.errPattern == "" {
			c.Assert(err, gc.IsNil)
			c.Assert(app, gc.NotNil)
		} else {
			c.Assert(err, gc.NotNil)
			c.Assert(err, gc.ErrorMatches, testCase.errPattern)
		}
	}
}
