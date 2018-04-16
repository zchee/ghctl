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
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/zchee/ghctl/pkg/spin"
)

// starCmd represents the star command
var starCmd = &cobra.Command{
	Use:   "star",
	Short: "manage the star",
}

var (
	starListCmd = &cobra.Command{
		Use:   "list",
		Short: "List the [username] starred repositories. If [username] is empty, use authenticated user by default",
		Run: func(cmd *cobra.Command, args []string) {
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
	rootCmd.AddCommand(starCmd)

	starCmd.AddCommand(starListCmd)
	starCmd.Flags().BoolVar(&starJSON, "json", false, "prints in JSON format instead of raw print")

	starListCmd.Flags().BoolVar(&starGitURL, "git", false, "print git url instead of HTML url")
	starListCmd.Flags().StringVar(&starListSort, "sort", "full_name", "Sort type of repositories to list. Default: full_name [created, updated, pushed, full_name]")
}

type starListResult struct {
	OwnerName string `json:"ownername"`
	URL       string `json:"url"`
}

func runStarList(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var starUsername string
	if len(args) > 0 {
		starUsername = args[0]
	}

	resultc, errc := listStarred(ctx, starUsername)

	s := spin.NewSpin()
	var results []starListResult
	i := 1
	for res := range resultc {
		s.Next("fetching", fmt.Sprintf("page: %d/%d", i, cap(resultc)))
		results = append(results, appendStarResult(res)...)
		i++
	}
	s.Flush()

	// handle first error
	if err := <-errc; err != nil {
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

func listStarred(ctx context.Context, username string) (<-chan []*github.StarredRepository, <-chan error) {
	client := newClient(ctx)
	options := &github.ActivityListStarredOptions{Sort: starListSort}
	options.Page = 1

	errc := make(chan error, 1)

	firstRepos, firstRes, err := client.Activity.ListStarred(ctx, username, options)
	if err != nil {
		err = checkRateLimitError(err)
		errc <- errors.Wrap(err, "could not get list starred")
		return nil, errc
	}
	if len(firstRepos) == 0 {
		errc <- errors.Errorf("%s user have not starred repository\n", username)
		return nil, errc
	}

	lastPage := firstRes.LastPage

	resultsc := make(chan []*github.StarredRepository, lastPage)
	resultsc <- firstRepos

	var errs []error
	sem := make(chan struct{}, 20)

	go func() {
		var wg sync.WaitGroup
		wg.Add(lastPage - 1)
		for i := 1; i < lastPage; i++ {
			sem <- struct{}{}
			go func(i int) {
				defer func() {
					<-sem
					wg.Done()
				}()

				opts := *options // copy
				opts.Page = i + 1
				repos, _, err := client.Activity.ListStarred(ctx, username, &opts)
				if err != nil {
					err = checkRateLimitError(err)
					errs = append(errs, errors.Wrap(err, "could not get list starred"))
					return
				}

				resultsc <- repos
			}(i)
		}

		go func() {
			wg.Wait()
			close(resultsc)
			if len(errs) > 0 {
				errc <- errs[0]
			}
			close(errc)
		}()
	}()

	return resultsc, errc
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
