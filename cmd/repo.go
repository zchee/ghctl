// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/github"
	"github.com/skratchdot/open-golang/open"
	cli "github.com/spf13/cobra"
	"github.com/pkg/errors"
)

// repoCmd represents the repo command
var repoCmd = &cli.Command{
	Use:   "repo",
	Short: "manage the repository",
}

var (
	repoListCmd = &cli.Command{
		Use:   "list",
		Short: "List the users repositories",
		Run: func(cmd *cli.Command, args []string) {
			if err := runRepoList(cmd, args); err != nil {
				cmd.Println(err)
			}
		},
	}
	repoDeleteCmd = &cli.Command{
		Use:   "delete",
		Short: "Delete repository",
		Run: func(cmd *cli.Command, args []string) {
			if err := runRepoDelete(cmd, args); err != nil {
				cmd.Println(err)
			}
		},
	}
	repoOpenCmd = &cli.Command{
		Use:   "open",
		Short: "Open repository",
		Run: func(cmd *cli.Command, args []string) {
			if err := runRepoOpen(cmd, args); err != nil {
				cmd.Println(err)
			}
		},
	}
)

func init() {
	RootCmd.AddCommand(repoCmd)

	repoCmd.AddCommand(repoListCmd)
	repoCmd.AddCommand(repoDeleteCmd)
	repoCmd.AddCommand(repoOpenCmd)

	repoListCmd.Flags().String("type", "all", "Type of repositories to list. Default: all [all, owner, public, private, member]")
}

func runRepoList(cmd *cli.Command, args []string) error {
	repoListType := cmd.Flag("type")
	var repoUsername string
	if len(args) > 0 {
		repoUsername = args[0]
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newClient(ctx)
	options := github.RepositoryListOptions{
		Type: repoListType.Value.String(),
	}
	options.Page = 1
	spin := newSpin()

	// pre-fetch page 1 for the get LastPage size
	firstRepos, firstRes, err := client.Repositories.List(ctx, repoUsername, &options)
	if err != nil {
		if _, ok := err.(*github.RateLimitError); ok {
			return errors.New("hit GitHub API rate limit")
		}
		if ctx.Err() != nil {
			return nil
		}
		return errors.Wrap(err, "could not get list all repositories")
	}

	lastPage := firstRes.LastPage
	if lastPage == 0 {
		return errors.Errorf("%s user have not %q repository", repoUsername, repoListType.Value.String())
	}

	// make lastPage size chan for parallel fetch
	repoURLsCh := make(chan []string, lastPage)

	// send first repository url to chan
	firstUrls := make([]string, len(firstRepos))
	for i, repo := range firstRepos {
		firstUrls[i] = repo.GetHTMLURL()
	}
	repoURLsCh <- firstUrls
	spin.next("fetching repository list", fmt.Sprintf("page: %d/%d", 0, lastPage))

	var wg sync.WaitGroup
	wg.Add(lastPage - 1)
	errs := make(chan error)
	sem := make(chan struct{}, 20)

	// alloc i to 1 because already fetched page 1
	for i := 1; i < lastPage; i++ {
		sem <- struct{}{}
		go func(opts github.RepositoryListOptions, i int) {
			defer func() {
				<-sem
				wg.Done()
			}()

			opts.Page = i + 1 // paging is based 1
			repos, _, err := client.Repositories.List(ctx, repoUsername, &opts)
			if err != nil {
				if _, ok := err.(*github.RateLimitError); ok {
					errs <- errors.New("hit GitHub API rate limit")
					return
				}
				errs <- errors.Wrap(err, "could not get list all repositories")
				return
			}

			urls := make([]string, len(repos))
			for j, repo := range repos {
				urls[j] = repo.GetHTMLURL()
			}
			repoURLsCh <- urls
			spin.next("fetching repository list", fmt.Sprintf("page: %d/%d", len(repoURLsCh), lastPage))
		}(options, i)
	}
	wg.Wait()
	close(repoURLsCh)
	spin.flush()

	if len(errs) != 0 {
		return <-errs
	}

	var repoURLs []string
	for rp := range repoURLsCh {
		repoURLs = append(repoURLs, rp...)
	}
	sort.Strings(repoURLs)

	fmt.Print(strings.Join(repoURLs, "\n"))

	return nil
}

func runRepoDelete(cmd *cli.Command, args []string) error {
	if err := checkArgs(cmd, args, 1, exactArgs, "<repository>"); err != nil {
		return err
	}
	repoDeleteName := args[0]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newClient(ctx)
	spin := newSpin()
	user, err := getUser(ctx, client)
	if err != nil {
		return errors.Wrap(err, "could not get user information")
	}

	fmt.Printf("remove repository %q? (y,n) ", repoDeleteName)
	r := bufio.NewReader(os.Stdin)
	ans, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(ans) != "y" {
		return errors.New("cancelled")
	}
	done := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				spin.next("deleting")
				time.Sleep(time.Millisecond)
			}
		}
	}()
	_, err = client.Repositories.Delete(ctx, user.GetLogin(), repoDeleteName)
	done <- struct{}{}
	spin.flush()
	if err != nil {
		return errors.Wrapf(err, "could not delete %s repository", repoDeleteName)
	}

	return nil
}

func runRepoOpen(cmd *cli.Command, args []string) error {
	if err := checkArgs(cmd, args, 1, exactArgs, "<username/repository>"); err != nil {
		return err
	}
	repoOpenName := args[0]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newClient(ctx)

	if !strings.Contains(repoOpenName, "/") {
		user, err := getUser(ctx, client)
		if err != nil {
			return errors.Wrap(err, "could not get user information")
		}
		repoOpenName = fmt.Sprintf("%s/%s", user.GetLogin(), repoOpenName)
	}

	u := fmt.Sprintf("https://github.com/%s", repoOpenName)
	resp, err := http.Get(u)
	if err != nil || resp.StatusCode == http.StatusNotFound {
		return errors.Errorf("failed http request: %s", u)
	}

	if err := open.Run(u); err != nil {
		return errors.Wrap(err, "could not open")
	}

	return nil
}
