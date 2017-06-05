// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/rhysd/locerr"
	"github.com/urfave/cli"
)

func init() {
	locerr.SetColor(true)
}

func main() {
	app := cli.NewApp()
	app.Name = "ghctl"
	app.Usage = "A CLI tool for GitHub repositories."
	app.Version = "0.0.1"
	app.Authors = []cli.Author{
		cli.Author{
			Name:  "zchee",
			Email: "<zchee.io@gmail.com>",
		},
	}
	app.Commands = []cli.Command{
		starCmd,
		repoCmd,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
}
