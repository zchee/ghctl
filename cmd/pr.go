// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v28/github"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/zchee/ghctl/pkg/spin"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// prCmd represents the pr command
var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "manage the pull request",
}

var (
	prListCmd = &cobra.Command{
		Use:   "list",
		Short: "List the your sent pull requests",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runPullRequestList(cmd, args); err != nil {
				cmd.Println(err)
			}
		},
	}

	prGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Gets you send pull requests from the specific repository",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runPullRequestGet(cmd, args); err != nil {
				cmd.Println(err)
			}
		},
	}
)

var (
	prIgnoreOwners []string
	prIgnoreRepos  []string
	prReverse      bool
	prMarkdown     bool
	prAll          bool

	prGetMarkdown bool
)

func init() {
	rootCmd.AddCommand(prCmd)

	prCmd.AddCommand(prListCmd)
	prCmd.AddCommand(prGetCmd)

	prListCmd.Flags().StringSliceVar(&prIgnoreOwners, "ignore-owner", nil, "ignore any owner repositories")
	prListCmd.Flags().StringSliceVar(&prIgnoreRepos, "ignore-repo", nil, "ignore any repository")
	prListCmd.Flags().BoolVar(&prReverse, "reverse", false, "reverse of sort order")
	prListCmd.Flags().BoolVarP(&prMarkdown, "markdown", "m", false, "output markdown syntax")
	prListCmd.Flags().BoolVarP(&prAll, "all", "a", false, "output all pull request (default: merged)")

	prGetCmd.Flags().BoolVarP(&prGetMarkdown, "markdown", "m", false, "output markdown syntax")
}

type pullRequestState string

const (
	pullRequestStateOpen   pullRequestState = "open"
	pullRequestStateClosed pullRequestState = "closed"
)

func runPullRequestList(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newClient(ctx)
	s := spin.NewSpin()

	user, err := getUser(ctx, client)
	if err != nil {
		return err
	}

	repos := []string{}
	if len(args) > 0 {
		repos = args
	}

	buf := new(bytes.Buffer)
	page := 1

	done := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				s.Next("fetching pull request list")
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()

	if err := getPullRequest(ctx, client, buf, user.GetLogin(), repos, pullRequestStateClosed, page); err != nil {
		return err
	}
	if prAll {
		if err := getPullRequest(ctx, client, buf, user.GetLogin(), repos, pullRequestStateOpen, page); err != nil {
			return err
		}
	}
	done <- struct{}{}
	s.Flush()

	fmt.Fprint(os.Stdout, buf.String())

	return nil
}

func getPullRequest(ctx context.Context, client *github.Client, buf io.Writer, username string, repos []string, state pullRequestState, page int) error {
	order := "asc"
	if prReverse {
		order = "desc"
	}
	options := &github.SearchOptions{
		Sort:  "updated",
		Order: order,
		ListOptions: github.ListOptions{
			Page: page,
		},
	}

	sep := " "
	query := "author:" + username + sep + "state:" + string(state) + sep + "type:pr"
	if len(repos) > 0 {
		for _, repo := range repos {
			if strings.Contains(repo, "/") {
				query += sep + "repo:" + repo
			} else {
				query += sep + "user:" + repo
			}
		}
	}
	prs, resp, err := client.Search.Issues(ctx, query, options)
	if err != nil {
		return errors.Wrap(IsRateLimitError(err), "could not get search pull request result")
	}

	for _, pr := range prs.Issues {
		// TODO(zchee): check flag whether the nil
		owner, repo := getRepoOwnerAndName(pr.GetURL())
		if matchSlice(owner, prIgnoreOwners) || matchSlice(repo, prIgnoreOwners) {
			continue
		}
		if prMarkdown {
			buf.Write([]byte(fmt.Sprintf("- [%s](%s)\n", pr.GetTitle(), pr.GetHTMLURL())))
			continue
		}
		buf.Write([]byte(fmt.Sprintf("url: %s, created: %s, title: %s\n", pr.GetHTMLURL(), pr.GetCreatedAt(), pr.GetTitle())))
	}

	if page == resp.LastPage || resp.NextPage == 0 {
		return nil
	}
	page = resp.NextPage

	return getPullRequest(ctx, client, buf, username, repos, state, page)
}

// getRepoOwnerAndName returns the repository owner and name.
// url assume github.Repository.GetURL() method result.
func getRepoOwnerAndName(url string) (string, string) {
	s := strings.TrimPrefix(url, "https://api.github.com/repos/")
	i := strings.IndexByte(s, '/')
	j := strings.IndexByte(s[i+1:], '/')
	return s[:i], s[i+1:][:j]
}

func matchSlice(s string, sepsl []string) bool {
	for _, sep := range sepsl {
		if s == sep {
			return true
		}
	}
	return false
}

func runPullRequestGet(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newClient(ctx)
	s := spin.NewSpin()

	owner := args[0]
	repo := args[1]

	done := make(chan struct{}, 1)
	go func(s *spin.Spin) {
		for {
			select {
			case <-done:
				return
			default:
				s.Next("fetching pull request list")
				time.Sleep(time.Second)
			}
		}
	}(s)

	prs, err := listPullRequests(ctx, client, owner, repo)
	if err != nil {
		return err
	}
	done <- struct{}{}
	s.Flush()

	builder := new(strings.Builder)
	for _, pr := range prs {
		if prGetMarkdown {
			builder.WriteString(fmt.Sprintf("- [%s](%s)\n", pr.GetTitle(), pr.GetHTMLURL()))
			continue
		}
		builder.WriteString(fmt.Sprintf("url: %s, created: %s, title: %s\n", pr.GetHTMLURL(), pr.GetCreatedAt(), pr.GetTitle()))
	}

	// fmt.Fprintf(os.Stdout, "prs: %s", spew.Sdump(prs))
	fmt.Fprintf(os.Stdout, builder.String())

	return nil
}

// listPullRequests lists the merged pull request from the github.com/owner/repo repository.
func listPullRequests(ctx context.Context, client *github.Client, owner string, repo string) ([]*github.PullRequest, error) {
	var reponame = owner + "/" + repo

	opts := &github.PullRequestListOptions{
		State:     "closed",
		Sort:      "created", // TODO(zchee): Needs set `Base` field for release branch name?
		Direction: "asc",     // TODO(zchee): desc?
	}

	prs, resp, err := client.PullRequests.List(ctx, owner, repo, opts)
	if err != nil {
		return nil, errors.Wrapf(IsRateLimitError(err), "failed get list of pull request from %s repository", reponame)
	}
	if code := resp.StatusCode; code != http.StatusOK {
		return nil, errors.Wrapf(err, "failed to get list of pull request from %s repository: status code %d", reponame, code)
	}

	lastPage := resp.LastPage
	if lastPage == 0 {
		return nil, errors.Errorf("not found pull requests from %s repository", reponame)
	}
	lastPage-- // decrease for already fetched page 1

	// make channel with size of lastPage for concurrency fetching
	prch := make(chan []*github.PullRequest, lastPage)
	go func() { prch <- prs }() // send first pull requests result

	// wg := new(sync.WaitGroup)
	// wg.Add(lastPage)
	// errc := make(chan error)

	var eg *errgroup.Group
	eg, ctx = errgroup.WithContext(ctx)
	sem := make(chan struct{}, 20) // for concurrency API access limit
	defer close(sem)

	fn := func(i int, opts *github.PullRequestListOptions) error {
		defer func() {
			<-sem
		}()

		opts.Page = i
		prs, resp, err := client.PullRequests.List(ctx, owner, repo, opts)
		if err != nil {
			return errors.WithStack(IsRateLimitError(err))
		}
		if code := resp.StatusCode; code != http.StatusOK {
			return errors.Wrapf(err, "failed to get %d pages pull requests from %s repository: status code %d", i, reponame, code)
		}

		prch <- prs
		return nil
	}

	for i := 0; i < lastPage; i++ {
		sem <- struct{}{}
		i := i
		copyopt := *opts // copy
		eg.Go(func() error {
			return fn(i, &copyopt)
		})
	}

	go func() {
		for {
			select {
			case pr := <-prch:
				prs = append(prs, pr...)
			case <-ctx.Done():
				close(prch)
				return
			}
		}
	}()

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return prs, nil
}

func listPullRequests2(ctx context.Context, client *github.Client, owner, repo string) ([]*github.PullRequest, error) {
	var reponame = owner + "/" + repo

	opts := &github.PullRequestListOptions{
		State:     "closed",
		Sort:      "created", // TODO(zchee): Needs set `Base` field for release branch name?
		Direction: "asc",     // TODO(zchee): desc?
	}

	prs, resp, err := client.PullRequests.List(ctx, owner, repo, opts)
	if err != nil {
		return nil, errors.Wrapf(IsRateLimitError(err), "failed get list of pull request from %s repository", reponame)
	}
	if code := resp.StatusCode; code != http.StatusOK {
		return nil, errors.Wrapf(err, "failed to get list of pull request from %s repository: status code %d", reponame, code)
	}

	lastPage := resp.LastPage
	if lastPage == 0 {
		return nil, errors.Errorf("not found pull requests from %s repository", reponame)
	}
	lastPage-- // decrease for already fetched page 1

	// make channel with size of lastPage for concurrency fetching
	prch := make(chan []*github.PullRequest, lastPage)
	go func() { prch <- prs }() // send first pull requests result

	var eg *errgroup.Group
	eg, ctx = errgroup.WithContext(ctx)

	sem := semaphore.NewWeighted(20) // for concurrency API access limit
	f := func(i int, opts *github.PullRequestListOptions) error {
		defer sem.Release(1)

		opts.Page = i
		prs, resp, err := client.PullRequests.List(ctx, owner, repo, opts)
		if err != nil {
			return errors.WithStack(IsRateLimitError(err))
		}
		if code := resp.StatusCode; code != http.StatusOK {
			return errors.Wrapf(err, "failed to get %d pages pull requests from %s repository: status code %d", i, reponame, code)
		}

		prch <- prs
		return nil
	}

	for i := 0; i < lastPage; i++ {
		var err error
		for err == nil {
			if err = sem.Acquire(ctx, 1); err != nil {
				fmt.Fprint(os.Stdout, "Failed to acquire semaphore: %v", err)
				continue
			}
		}

		i := i
		copyopt := *opts // copy
		eg.Go(func() error {
			return f(i, &copyopt)
		})
	}

	go func() {
		for {
			select {
			case pulls := <-prch:
				prs = append(prs, pulls...)
			case <-ctx.Done():
				close(prch)
				return
			}
		}
	}()

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return prs, nil
}
