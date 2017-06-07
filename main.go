// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli"
)

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
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "profile",
			Usage: "write CPU profile to file",
		},
	}

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprint(os.Stderr, err)
	os.Exit(1)
}
