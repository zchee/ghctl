// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"text/tabwriter"

	"github.com/google/go-github/github"
	cli "github.com/spf13/cobra"
	"github.com/zchee/ghctl/internal/errors"
)

// starCmd represents the star command
var starCmd = &cli.Command{
	Use:   "star",
	Short: "manage the star",
}

var (
	starListCmd = &cli.Command{
		Use:   "list",
		Short: "List the [username] starred repositories. If [username] is empty, use authenticated user by default",
		Run: func(cmd *cli.Command, args []string) {
			if err := runStarList(cmd, args); err != nil {
				cmd.Println(err)
			}
		},
	}
)

var (
	starGitURL bool
	starJSON   bool
)

func init() {
	RootCmd.AddCommand(starCmd)

	starCmd.AddCommand(starListCmd)
	starCmd.Flags().BoolVar(&starJSON, "json", false, "prints in JSON format instead of raw print")

	starListCmd.Flags().BoolVar(&starGitURL, "git", false, "print git url instead of HTML url")
}

type starListResult struct {
	OwnerName string `json:"ownername"`
	URL       string `json:"url"`
}

func runStarList(cmd *cli.Command, args []string) error {
	var starUsername string
	if len(args) > 0 {
		starUsername = args[0]
	}

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
			spin.next("fetching", fmt.Sprintf("page: %d/%d", len(resultsCh), lastPage))
		}(i)
	}
	wg.Wait()
	close(resultsCh)
	spin.flush()

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
