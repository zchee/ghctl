// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"

	"github.com/google/go-github/github"
	"github.com/urfave/cli"
	"github.com/zchee/ghctl/internal/errors"
)

var repoCmd = cli.Command{
	Name:  "repo",
	Usage: "manage the repository.",
	Subcommands: []cli.Command{
		repoListCmd,
	},
}

var repoListCmd = cli.Command{
	Name:      "list",
	Usage:     "List the users repositories.",
	ArgsUsage: "[username]",
	Before:    initRepoList,
	Action:    runRepoList,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "type",
			Usage: "Type of repositories to list. Default: all [all, owner, public, private, member]",
		},
	},
}

var (
	repoUsername string
	repoListType string
)

func initRepoList(c *cli.Context) error {
	repoUsername = c.Args().First()
	repoListType = c.String("type")

	return nil
}

func runRepoList(c *cli.Context) error {
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
	options := &github.RepositoryListOptions{
		Type: repoListType,
	}
	options.Page = 1
	spin := newSpin()

	// pre-fetch page 1 for the get LastPage size
	firstRepos, firstRes, err := client.Repositories.List(ctx, repoUsername, options)
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
		return errors.Errorf("%s user have not %q repository", repoUsername, repoListType)
	}
	spin.next("fetching repository list", fmt.Sprintf("page: %d/%d", 0, lastPage))

	// make lastPage size chan for parallel fetch
	repoURLsCh := make(chan []string, lastPage)

	// send first repositoriy urls to chan
	firstUrls := make([]string, len(firstRepos))
	for i, repo := range firstRepos {
		firstUrls[i] = repo.GetHTMLURL()
	}
	repoURLsCh <- firstUrls

	var wg sync.WaitGroup
	errs := make(chan error)
	// alloc i to 1 because already fetched page 1
	for i := 1; i < lastPage; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			opts := *options  // copy
			opts.Page = i + 1 // paging is based 1

			repos, _, err := client.Repositories.List(ctx, repoUsername, &opts)
			if err != nil {
				if _, ok := err.(*github.RateLimitError); ok {
					errs <- errors.New("hit GitHub API rate limit")
					return
				}
				if ctx.Err() != nil {
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
		}(i)
	}

	var repoURLs []string
	// add 1 to WaitGroup counter because repoURLs is data racy.
	wg.Add(1)
	// run goroutine for spinner and append repoURL results
	go func() {
		defer wg.Done()

		var j int
		for {
			select {
			case urls := <-repoURLsCh:
				j++
				repoURLs = append(repoURLs, strings.Join(urls, "\n"))
				spin.next("fetching repository list", fmt.Sprintf("page: %d/%d", j, lastPage))
				if j == lastPage {
					sort.Strings(repoURLs)
					spin.flush()
					return
				}
			}
		}
	}()
	// wait for parallel fetching and gather repoURL results
	wg.Wait()

	if len(errs) != 0 {
		// return first error only
		return <-errs
	}

	var out bytes.Buffer
	for _, r := range repoURLs {
		out.WriteString(r)
		out.WriteByte('\n')
	}

	fmt.Print(out.String())

	return nil
}
