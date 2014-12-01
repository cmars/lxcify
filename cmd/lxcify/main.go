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
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/juju/errors"

	"github.com/cmars/lxcify/template"
)

var (
	config string
	name   string
)

func init() {
	flag.StringVar(&config, "config", "", "app config file")
	flag.StringVar(&name, "name", "", "container name")
}

func die(err error) {
	if err != nil {
		log.Fatalln(err)
	}
	os.Exit(0)
}

func parseFlags() {
	flag.Parse()
	if config == "" {
		log.Println("missing required flag -config")
		usage()
	}
	if name == "" {
		log.Println("missing required flag -name")
		usage()
	}
}

func usage() {
	flag.PrintDefaults()
	os.Exit(1)
}

func main() {
	parseFlags()

	err := run()
	if err != nil {
		die(err)
	}
}

func run() error {
	var conf []byte
	var err error

	f, err := os.Open(config)
	if err != nil {
		return errors.Trace(err)
	}
	defer f.Close()
	conf, err = ioutil.ReadAll(f)
	if err != nil {
		return errors.Trace(err)
	}

	t, err := template.Parse(conf)
	if err != nil {
		return errors.Trace(err)
	}

	c, err := t.Container(name)
	if err != nil {
		return errors.Trace(err)
	}
	app, err := t.App()
	if err != nil {
		return errors.Trace(err)
	}

	err = c.Create()
	if err != nil {
		return errors.Trace(err)
	}

	err = c.Start()
	if err != nil {
		return errors.Trace(err)
	}

	err = c.Install(app)
	if err != nil {
		return errors.Trace(err)
	}

	err = c.Stop()
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}
