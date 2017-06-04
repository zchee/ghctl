// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"text/tabwriter"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var subFlagsList = []cli.Flag{
	cli.BoolFlag{
		Name:  "json, j",
		Usage: "prints in the JSON format instead of simple print.",
	},
	cli.BoolFlag{
		Name:  "verbose, v",
		Usage: "be verbose",
	},
	cli.BoolFlag{
		Name:  "quiet, q",
		Usage: "suppress some output",
	},
}

var (
	listStarredCmd = cli.Command{
		Name:      "star",
		Usage:     "List the user starred repositories.",
		ArgsUsage: "[username]",
		Before:    initListStarred,
		Action:    runListStarred,
		Flags: append(subFlagsList,
			cli.BoolFlag{
				Name:  "git, g",
				Usage: "print git url instead of HTML url",
			}),
	}
)

var listCmd = cli.Command{
	Name:  "list",
	Usage: "List the repositories.",
	Subcommands: []cli.Command{
		listStarredCmd,
	},
	Flags: subFlagsList,
}

var (
	listUsername string

	listGitURL  bool
	listJSON    bool
	listQuiet   bool
	listVerbose bool
)

type listResult struct {
	OwnerName string `json:"ownername"`
	URL       string `json:"url"`
}

func initListStarred(c *cli.Context) error {
	listUsername = c.Args().First()

	listGitURL = c.Bool("git")
	listJSON = c.GlobalBool("json") || c.Bool("json")
	listQuiet = c.GlobalBool("quiet") || c.Bool("quiet")
	listVerbose = c.GlobalBool("verbose") || c.Bool("verbose")

	return nil
}

func runListStarred(c *cli.Context) error {
	ctx, cancel := context.WithCancel(context.Background())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer func() {
		signal.Stop(sig)
		cancel()
	}()
	go func() {
		select {
		case <-sig:
			cancel()
		case <-ctx.Done():
		}
	}()

	client := newClient(ctx)
	options := &github.ActivityListStarredOptions{Sort: "created"}
	spin := newSpin()

	var results []listResult
	for i := 0; ; i++ {
		options.Page = i
		repos, res, err := client.Activity.ListStarred(ctx, listUsername, options)
		if err != nil {
			spin.flush()
			if _, ok := err.(*github.RateLimitError); ok {
				return errors.New("hit GitHub API rate limit")
			}
			if ctx.Err() != nil {
				return nil
			}
			return errors.Wrap(err, "could not get list starred")
		}

		spin.next("fetching", fmt.Sprintf("page: %d/%d", i+1, res.LastPage))

		for _, repo := range repos {
			res := listResult{
				OwnerName: repo.Repository.GetFullName(),
			}
			if listGitURL {
				res.URL = repo.Repository.GetGitURL()
			} else {
				res.URL = repo.Repository.GetURL()
			}
			results = append(results, res)
		}
		if i >= res.LastPage {
			break
		}
	}
	spin.flush()

	if len(results) == 0 {
		return errors.Errorf("%s user have not starred repository\n", listUsername)
	}

	if listJSON {
		buf, err := json.MarshalIndent(results, "", "\t")
		if err != nil {
			return errors.Wrap(err, "could not marshal to JSON")
		}
		fmt.Print(string(buf))
	} else {
		w := tabwriter.NewWriter(os.Stdout, 0, 8, 0, '\t', tabwriter.AlignRight)
		for _, res := range results {
			fmt.Fprintln(w, fmt.Sprintf("owner: %s\turl: %s", res.OwnerName, res.URL))
		}
		w.Flush()
	}

	return nil
}
