// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	starGitURL   bool
	starListSort string
	starJSON     bool
)

func init() {
	RootCmd.AddCommand(starCmd)

	starCmd.AddCommand(starListCmd)
	starCmd.Flags().BoolVar(&starJSON, "json", false, "prints in JSON format instead of raw print")

	starListCmd.Flags().BoolVar(&starGitURL, "git", false, "print git url instead of HTML url")
	starListCmd.Flags().StringVar(&starListSort, "sort", "full_name", "Sort type of repositories to list. Default: full_name [created, updated, pushed, full_name]")
}

type starListResult struct {
	OwnerName string `json:"ownername"`
	URL       string `json:"url"`
}

func runStarList(cmd *cli.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var starUsername string
	if len(args) > 0 {
		starUsername = args[0]
	}

	results, err := starList(ctx, starUsername)
	if err != nil {
		return err
	}

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

func starList(ctx context.Context, username string) ([]starListResult, error) {
	client := newClient(ctx)
	options := &github.ActivityListStarredOptions{Sort: starListSort}
	options.Page = 1
	spin := newSpin()

	firstRepos, firstRes, err := client.Activity.ListStarred(ctx, username, options)
	if err != nil {
		if _, ok := err.(*github.RateLimitError); ok {
			return nil, ErrRateLimit
		}
		return nil, errors.Wrap(err, "could not get list starred")
	}
	if len(firstRepos) == 0 {
		return nil, errors.Errorf("%s user have not starred repository\n", username)
	}

	lastPage := firstRes.LastPage

	resultsCh := make(chan []starListResult, lastPage)
	resultsCh <- appendStarResult(firstRepos)
	spin.next("fetching", fmt.Sprintf("page: %d/%d", 1, lastPage))

	var wg sync.WaitGroup
	wg.Add(lastPage - 1)
	errs := make(chan error)
	for i := 1; i < lastPage; i++ {
		go func(i int) {
			defer wg.Done()

			opts := *options // copy
			opts.Page = i + 1
			repos, _, err := client.Activity.ListStarred(ctx, username, &opts)
			if err != nil {
				if _, ok := err.(*github.RateLimitError); ok {
					errs <- errors.New("hit GitHub API rate limit")
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
		return nil, <-errs
	}

	var results []starListResult
	for r := range resultsCh {
		results = append(results, r...)
	}

	return results, nil
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
