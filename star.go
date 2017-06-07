// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"text/tabwriter"

	"github.com/google/go-github/github"
	"github.com/urfave/cli"
	"github.com/zchee/ghctl/internal/errors"
)

var starCmd = cli.Command{
	Name:  "star",
	Usage: "manage the star",
	Subcommands: []cli.Command{
		starListCmd,
	},
	Flags: starSubFlagsList,
}

var starListCmd = cli.Command{
	Name:      "list",
	Usage:     "List the [username] starred repositories. If [username] is empty, use authenticated user by default",
	ArgsUsage: "[username]",
	Before:    initStarList,
	Action:    runStarList,
	Flags: append(starSubFlagsList,
		cli.BoolFlag{
			Name:  "git, g",
			Usage: "print git url instead of HTML url",
		}),
}

var starSubFlagsList = []cli.Flag{
	cli.BoolFlag{
		Name:  "json, j",
		Usage: "prints in JSON format instead of raw print",
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
	starUsername string

	starGitURL  bool
	starJSON    bool
	starQuiet   bool
	starVerbose bool
)

type starListResult struct {
	OwnerName string `json:"ownername"`
	URL       string `json:"url"`
}

func initStarList(c *cli.Context) error {
	starUsername = c.Args().First()

	starGitURL = c.Bool("git")
	starJSON = c.GlobalBool("json") || c.Bool("json")
	starQuiet = c.GlobalBool("quiet") || c.Bool("quiet")
	starVerbose = c.GlobalBool("verbose") || c.Bool("verbose")

	return nil
}

func runStarList(c *cli.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newClient(ctx)
	options := &github.ActivityListStarredOptions{Sort: "created"}
	options.Page = 1
	spin := newSpin()

	firstRepos, firstRes, err := client.Activity.ListStarred(ctx, starUsername, options)
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
	if len(firstRepos) == 0 {
		return errors.Errorf("%s user have not starred repository\n", starUsername)
	}

	lastPage := firstRes.LastPage

	resultsCh := make(chan []starListResult, lastPage)
	resultsCh <- appendStarResult(firstRepos)
	go spin.next("fetching", fmt.Sprintf("page: %d/%d", 1, lastPage))

	var wg sync.WaitGroup
	wg.Add(lastPage - 1)
	errs := make(chan error)
	for i := 1; i < lastPage; i++ {
		go func(i int) {
			defer wg.Done()

			opts := *options // copy
			opts.Page = i + 1
			repos, _, err := client.Activity.ListStarred(ctx, starUsername, &opts)
			if err != nil {
				spin.flush()
				if _, ok := err.(*github.RateLimitError); ok {
					errs <- errors.New("hit GitHub API rate limit")
					return
				}
				if ctx.Err() != nil {
					return
				}
				errs <- errors.Wrap(err, "could not get list starred")
				return
			}

			resultsCh <- appendStarResult(repos)
			go spin.next("fetching", fmt.Sprintf("page: %d/%d", len(resultsCh), lastPage))
		}(i)
	}
	wg.Wait()
	close(resultsCh)

	if len(errs) != 0 {
		return <-errs
	}

	var results []starListResult
	for r := range resultsCh {
		results = append(results, r...)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].OwnerName < results[j].OwnerName
	})
	if starJSON {
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
		if err := w.Flush(); err != nil {
			return errors.Wrap(err, "could not flush tabwriter")
		}
	}

	return nil
}

func appendStarResult(repos []*github.StarredRepository) []starListResult {
	var results []starListResult
	for _, repo := range repos {
		res := starListResult{
			OwnerName: repo.Repository.GetFullName(),
		}
		if starGitURL {
			res.URL = repo.Repository.GetGitURL()
		} else {
			res.URL = repo.Repository.GetHTMLURL()
		}
		results = append(results, res)
	}
	return results
}
