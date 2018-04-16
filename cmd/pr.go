// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/zchee/ghctl/pkg/spin"
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
)

var (
	prIgnoreOwners []string
	prIgnoreRepos  []string
	prReverse      bool
	prMarkdown     bool
	prAll          bool
)

func init() {
	RootCmd.AddCommand(prCmd)

	prCmd.AddCommand(prListCmd)

	prListCmd.Flags().StringSliceVar(&prIgnoreOwners, "ignore-owner", nil, "ignore any owner repositories")
	prListCmd.Flags().StringSliceVar(&prIgnoreRepos, "ignore-repo", nil, "ignore any repository")
	prListCmd.Flags().BoolVar(&prReverse, "reverse", false, "reverse of sort order")
	prListCmd.Flags().BoolVarP(&prMarkdown, "markdown", "m", false, "output markdown syntax")
	prListCmd.Flags().BoolVarP(&prAll, "all", "a", false, "output all pull request (default: merged)")
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
		return errors.Wrap(checkRateLimitError(err), "could not get search pull request result")
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
