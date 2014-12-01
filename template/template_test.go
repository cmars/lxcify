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
	gc "launchpad.net/gocheck"
	stdtesting "testing"
)

func Test(t *stdtesting.T) {
	gc.TestingT(t)
}

type ConfigSuite struct{}

var _ = gc.Suite(&ConfigSuite{})

var testYaml = `
container:
  template: ubuntu

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

func (*ConfigSuite) TestContent(c *gc.C) {
	t, err := Parse([]byte(testYaml))
	c.Assert(err, gc.IsNil)
	c.Assert(t, gc.NotNil)

	c.Assert(t.Mounts, gc.HasLen, 2)
	c.Assert(t.Mounts[0], gc.DeepEquals,
		mount{
			Passthru: "/dev/dri",
			IsDir:    true,
		})
	c.Assert(t.Mounts[1], gc.DeepEquals,
		mount{
			Host:      "/dev/video1",
			Container: "/dev/video0",
			IsDir:     false,
		})
	c.Assert(t.SharePulseAudio, gc.Equals, true)
	c.Assert(t.InstallScript, gc.Matches, "(?m).*apt-get update.*")
	c.Assert(t.LaunchCommand, gc.Equals, "/bin/beef")
	c.Assert(t.DesktopLauncher, gc.NotNil)
	c.Assert(t.DesktopLauncher.Name, gc.Equals, "beef")
	c.Assert(t.DesktopLauncher.Comment, gc.Equals, "beefy beef")
	c.Assert(t.DesktopLauncher.IconPath, gc.Matches, ".*ribeye[.]png$")
	c.Assert(t.DesktopLauncher.Categories, gc.HasLen, 2)
	c.Assert(t.DesktopLauncher.Categories[0], gc.Equals, "mmm")
	c.Assert(t.DesktopLauncher.Categories[1], gc.Equals, "beef")
}

func (*ConfigSuite) TestYamlParse(c *gc.C) {
	testCases := []struct {
		yaml       string
		errPattern string
	}{{
		yaml: testYaml,
	}, {
		yaml:       "}{",
		errPattern: "YAML error:.*",
	}}

	for i, testCase := range testCases {
		c.Log("test#", i)
		t, err := Parse([]byte(testCase.yaml))
		if testCase.errPattern == "" {
			c.Assert(err, gc.IsNil)
			c.Assert(t, gc.NotNil)
		} else {
			c.Assert(err, gc.NotNil)
			c.Assert(err, gc.ErrorMatches, testCase.errPattern)
		}
	}
}

func (*ConfigSuite) TestAppReqFields(c *gc.C) {
	testCases := []struct {
		yaml       string
		errPattern string
	}{{
		yaml: testYaml,
	}, {
		yaml:       "nope:nope:nope:",
		errPattern: "missing install-script",
	}, {
		yaml:       `install-script: a`,
		errPattern: "missing launch-command",
	}, {
		yaml:       `{install-script: a, launch-command: b}`,
		errPattern: "",
	}}

	for i, testCase := range testCases {
		c.Log("test#", i)
		t, err := Parse([]byte(testCase.yaml))
		c.Assert(err, gc.IsNil)
		app, err := t.App()
		if testCase.errPattern == "" {
			c.Assert(err, gc.IsNil)
			c.Assert(app, gc.NotNil)
		} else {
			c.Assert(err, gc.NotNil)
			c.Assert(err, gc.ErrorMatches, testCase.errPattern)
		}
	}
}

func (*ConfigSuite) TestCreateContainer(c *gc.C) {
	t, err := Parse([]byte(testYaml))
	c.Assert(err, gc.IsNil)
	c.Assert(t, gc.NotNil)

	container, err := t.Container("foo")
	c.Assert(err, gc.IsNil)

	c.Assert(container.Name(), gc.Equals, "foo")
}
