// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"fmt"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var listCmd = cli.Command{
	Name:  "list",
	Usage: "List the repositories.",
	Subcommands: []cli.Command{
		cli.Command{
			Name:      "star",
			Usage:     "List the user starred repositories.",
			Before:    initListStarred,
			Action:    runListStarred,
			ArgsUsage: "<username>",
		},
	},
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "be verbose",
		},
	},
}

var (
	listUsername string
	listVerbose  bool
)

func initListStarred(c *cli.Context) error {
	listVerbose = c.GlobalBool("verbose")

	if err := checkArgs(c, 1, exactArgs, "<username>"); err != nil {
		return err
	}
	listUsername = c.Args().First()
	return nil
}

func runListStarred(c *cli.Context) error {
	options := &github.ActivityListStarredOptions{Sort: "created"}
	client := newClient()

	var buf bytes.Buffer
	spin := NewSpin()
	for i := 0; ; i++ {
		options.Page = i

		repos, res, err := client.Activity.ListStarred(context.Background(), listUsername, options)
		if err != nil {
			return errors.Wrap(err, "could not get list starred")
		}

		spin.Next("fetching", fmt.Sprintf("page: %d/%d", i, res.LastPage))

		for _, repo := range repos {
			buf.WriteString(*repo.Repository.HTMLURL + "\n")
		}

		if i >= res.LastPage {
			break
		}
	}

	if buf.Len() == 0 {
		return errors.Errorf("%s user have not starred repository", listUsername)
	}

	fmt.Print(buf.String())
	return nil
}
