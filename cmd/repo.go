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

	"github.com/google/go-github/v21/github"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	open "github.com/zchee/go-open"
	"golang.org/x/sync/errgroup"

	errpkg "github.com/zchee/ghctl/pkg/errors"
	"github.com/zchee/ghctl/pkg/spin"
)

// repoCmd represents the repo command.
var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "manage the repository",
}

var (
	repoListCmd = &cobra.Command{
		Use:   "list <username|orgs>",
		Short: "List the users repositories",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runRepoList(cmd, args); err != nil {
				fmt.Fprint(cmd.OutOrStderr(), err)
			}
		},
	}
	repoDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete repository",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runRepoDelete(cmd, args); err != nil {
				fmt.Fprint(cmd.OutOrStderr(), err)
			}
		},
	}
	repoOpenCmd = &cobra.Command{
		Use:   "open",
		Short: "Open repository",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runRepoOpen(cmd, args); err != nil {
				fmt.Fprint(cmd.OutOrStderr(), err)
			}
		},
	}
	repoCollaboratorCmd = &cobra.Command{
		Use:   "collaborator",
		Short: "manage repository's collaborators.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runRepoCollaborator(cmd, args); err != nil {
				fmt.Fprint(cmd.OutOrStderr(), err)
			}
		},
	}
	repoAcceptInvitationCmd = &cobra.Command{
		Use:   "accept <owner/repository>",
		Short: "accept collaborator invitation",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runRepoAcceptInvitation(cmd, args); err != nil {
				fmt.Fprint(cmd.OutOrStderr(), err)
			}
		},
	}
)

type repoFlags struct {
	typ         string
	affiliation string

	collaborator string

	acceptUserToken string
}

var (
	flags = &repoFlags{}
)

func init() {
	rootCmd.AddCommand(repoCmd)

	repoCmd.AddCommand(repoListCmd)
	repoListCmd.Flags().StringVarP(&flags.typ, "type", "t", "all", "Type of repositories to list. Default: all [all, owner, public, private, member]")
	repoListCmd.Flags().StringVarP(&flags.affiliation, "affiliation", "a", "", "Comma separated list repos of given affiliation[s]. [owner,collaborator,organization_member]")

	repoCmd.AddCommand(repoDeleteCmd)

	repoCmd.AddCommand(repoOpenCmd)

	repoCmd.AddCommand(repoCollaboratorCmd)
	repoCollaboratorCmd.Flags().StringVar(&flags.collaborator, "collaborator", "", "username of collaborator.")

	repoCmd.AddCommand(repoAcceptInvitationCmd)
	repoAcceptInvitationCmd.Flags().StringVar(&flags.acceptUserToken, "token", "", "GitHub TOKEN for accepting user is different.")
}

func runRepoList(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newClient(ctx)
	s := spin.NewSpin()

	opts := github.RepositoryListOptions{
		Type: flags.typ,
		ListOptions: github.ListOptions{
			Page: 1,
		},
	}
	switch flags.typ {
	case "public", "private":
		opts.Visibility = flags.typ
	}
	if flags.affiliation != "" {
		opts.Affiliation = flags.affiliation
	}

	var repoName string
	// If empty, use login user
	if len(args) > 0 {
		repoName = args[0]
	}
	// pre-fetch page 1 for the get LastPage size
	firstRepos, firstResp, err := client.Repositories.List(ctx, repoName, &opts)
	if err != nil {
		if errpkg.IsRateLimitErr(err) {
			return errors.New("repo: hit GitHub API rate limit")
		}
		if ctx.Err() != nil {
			return nil
		}
		return errors.Wrap(err, "repo: could not get list all repositories")
	}

	lastPage := firstResp.LastPage
	if lastPage == 0 {
		return errors.Errorf("repo: %s user have not %q repository", repoName, flags.typ)
	}

	// make lastPage size chan for parallel fetch
	reposCh := make(chan []string, lastPage)
	uris := make([]string, len(firstRepos))
	for i, repo := range firstRepos {
		uris[i] = repo.GetHTMLURL()
	}
	reposCh <- uris
	s.Next(spin.FetchMsg, fmt.Sprintf("page: %d/%d", 0, lastPage))

	wg := new(sync.WaitGroup)
	wg.Add(lastPage - 1)
	sem := make(chan struct{}, 20)
	errs := make(chan error)

	// alloc i to 1 because already fetched page 1
	for i := 1; i < lastPage; i++ {
		sem <- struct{}{}
		go func(opts github.RepositoryListOptions, i int) {
			defer func() {
				<-sem
				wg.Done()
			}()

			opts.Page = i + 1 // paging is based 1
			repos, _, err := client.Repositories.List(ctx, repoName, &opts)
			if err != nil {
				if errpkg.IsRateLimitErr(err) {
					errs <- errpkg.ErrRateLimit
					return
				}
				errs <- errors.Wrap(err, "repo: could not get list all repositories")
				return
			}

			urls := make([]string, len(repos))
			for j, repo := range repos {
				urls[j] = repo.GetHTMLURL()
			}
			reposCh <- urls
			s.Next(spin.FetchMsg, fmt.Sprintf("page: %d/%d", len(reposCh), lastPage))
		}(opts, i)
	}

	go func() {
		wg.Wait()
		close(reposCh)
		s.Flush()
	}()

	if len(errs) != 0 {
		return <-errs
	}

	var repos []string
	for r := range reposCh {
		repos = append(repos, r...)
	}
	sort.Strings(repos)

	fmt.Print(strings.Join(repos, "\n"))

	return nil
}

func runRepoDelete(cmd *cobra.Command, args []string) error {
	if err := checkArgs(cmd, args, 1, exactArgs, "<repository>"); err != nil {
		return err
	}
	repoDeleteName := args[0]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newClient(ctx)
	s := spin.NewSpin()
	user, err := getUser(ctx, client)
	if err != nil {
		return errors.Wrap(err, "could not get user information")
	}

	fmt.Printf("remove repository %q? (y,n) ", repoDeleteName)
	r := bufio.NewReader(os.Stdin)
	confirm, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	if strings.TrimSpace(confirm) != "y" {
		return errors.New("cancelled")
	}
	done := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				s.Next("deleting")
				time.Sleep(time.Millisecond)
			}
		}
	}()
	_, err = client.Repositories.Delete(ctx, user.GetLogin(), repoDeleteName)
	done <- struct{}{}
	s.Flush()
	if err != nil {
		return errors.Wrapf(err, "could not delete %s repository", repoDeleteName)
	}

	return nil
}

func runRepoOpen(cmd *cobra.Command, args []string) error {
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

func runRepoCollaborator(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := checkArgs(cmd, args, 1, exactArgs, "<owner/repository>"); err != nil {
		return err
	}
	ss := strings.Split(args[0], "/")
	owner := ss[0]
	repo := ss[1]
	if flags.collaborator == "" {
		return errors.New("--collaborator flag must be not empty")
	}
	collaborator := flags.collaborator

	client := newClient(ctx)
	resp, err := client.Repositories.AddCollaborator(ctx, owner, repo, collaborator, &github.RepositoryAddCollaboratorOptions{Permission: "admin"})
	if err != nil {
		return errors.Wrap(IsRateLimitError(err), "repo: could not get list all repositories")
	}
	if resp.StatusCode == http.StatusNoContent {
		return fmt.Errorf("%s user already collaborator on %s/%s", collaborator, owner, repo)
	}

	fmt.Fprintf(os.Stdout, "added %s user to %s/%s collaborator\n", collaborator, owner, repo)

	return nil
}

func runRepoAcceptInvitation(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := checkArgs(cmd, args, 1, exactArgs, "<owner/repository>"); err != nil {
		return err
	}

	client := newClientFromToken(ctx, flags.acceptUserToken)
	opts := &github.ListOptions{
		Page:    1,
		PerPage: 100,
	}
	invitations, resp, err := client.Users.ListInvitations(ctx, opts)
	if err != nil {
		return errors.Wrap(IsRateLimitError(err), "repo: could not get list invitations")
	}

	lastPage := resp.LastPage
	lastPage-- // decrease for already gets page 1

	if lastPage > 0 {
		// make channel with size of lastPage for concurrency fetching
		invch := make(chan []*github.RepositoryInvitation, lastPage)
		go func() { invch <- invitations }() // send first API call results

		var eg *errgroup.Group
		eg, ctx = errgroup.WithContext(ctx)

		sem := make(chan struct{}, 20) // for concurrency API access limit
		defer close(sem)

		f := func(page int, opts *github.ListOptions) error {
			opts.Page = page
			invs, resp, err := client.Users.ListInvitations(ctx, opts)
			if err != nil {
				return errors.WithStack(IsRateLimitError(err))
			}
			if code := resp.StatusCode; code != http.StatusOK {
				return errors.Wrapf(err, "failed to get %d pages invitation: status code %d", page, code)
			}

			invch <- invs
			<-sem
			return nil
		}

		for i := 0; i < lastPage; i++ {
			sem <- struct{}{}

			i := i
			copyopt := *opts // copy
			eg.Go(func() error {
				return f(i, &copyopt)
			})
		}

		go func() {
			for {
				select {
				case invs := <-invch:
					invitations = append(invitations, invs...)
				case <-ctx.Done():
					close(invch)
					return
				}
			}
		}()

		if err := eg.Wait(); err != nil {
			return err
		}
	}

	fullname := args[0]
	var invID int64
	for _, inv := range invitations {
		if strings.EqualFold(inv.GetRepo().GetFullName(), fullname) {
			invID = inv.GetID()
		}
	}
	if invID == 0 {
		return errors.Errorf("repo: not found invitation from %s repository", fullname)
	}

	respAccept, err := client.Users.AcceptInvitation(ctx, invID)
	if err != nil {
		return errors.WithStack(IsRateLimitError(err))
	}
	if code := respAccept.StatusCode; code != http.StatusNoContent {
		return errors.Errorf("repo: failed to accept %d invitation: status: %s", invID, http.StatusText(code))
	}

	fmt.Fprintf(os.Stdout, "accepted %d invitation ID from %s repository\n", invID, fullname)

	return nil
}
