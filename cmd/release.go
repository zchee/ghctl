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
	"strings"

	"github.com/google/go-github/v24/github"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// releaseCmd represents the release command.
var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "manage the repository releases",
}

var (
	releaseCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "create any repository release",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runReleaseCreate(cmd, args); err != nil {
				cmd.Println(err)
			}
		},
	}

	releaseDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete any repository release",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runReleaseDelete(cmd, args); err != nil {
				cmd.Println(err)
			}
		},
	}
)

var (
	releaseDeleteWithTag bool
	releaseDeleteForce bool
)

func init() {
	rootCmd.AddCommand(releaseCmd)

	releaseCmd.AddCommand(releaseCreateCmd)
	releaseCmd.AddCommand(releaseDeleteCmd)

	releaseDeleteCmd.Flags().BoolVar(&releaseDeleteWithTag, "with-tag", false, "delete also tag")
	releaseDeleteCmd.Flags().BoolVarP(&releaseDeleteForce, "force", "f", false, "force deleting")
}

func runReleaseCreate(cmd *cobra.Command, args []string) error {
	if err := checkArgs(cmd, args, 3, exactArgs, "<owner> <repo> <tag>"); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	owner := args[0]
	repo := args[1]
	tag := args[2]

	client := newClient(ctx)
	body := fmt.Sprintf("Release %s.", tag)
	_, resp, err := client.Repositories.CreateRelease(ctx, owner, repo, &github.RepositoryRelease{
		TagName: &tag,
		Name:    &tag,
		Body:    &body,
	})
	if err != nil {
		return errors.Wrapf(checkRateLimitError(err), "could not create %s release to %s/%s", tag, owner, repo)
	}
	if resp.StatusCode != http.StatusOK {
		return errors.Wrap(err, "failed")
	}

	fmt.Fprintf(os.Stdout, "Created %s release\n", tag)

	return nil
}

func runReleaseDelete(cmd *cobra.Command, args []string) error {
	if err := checkArgs(cmd, args, 3, exactArgs, "<owner> <repo> <tag>"); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	owner := args[0]
	repo := args[1]
	tag := args[2]

	client := newClient(ctx)
	released, resp, err := client.Repositories.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		return errors.Wrapf(checkRateLimitError(err), "could not create %s release to %s/%s", tag, owner, repo)
	}
	if resp.StatusCode != http.StatusOK {
		return errors.Wrap(err, "failed")
	}

	if !releaseDeleteForce {
		fmt.Printf("delete %q release? (y,n): ", owner+"/"+repo+"/"+tag)
		r := bufio.NewReader(os.Stdin)
		confirm, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.TrimSpace(confirm) != "y" {
			return errors.New("cancelled")
		}
	}

	resp, err = client.Repositories.DeleteRelease(ctx, owner, repo, released.GetID())
	if err != nil {
		return errors.Wrapf(checkRateLimitError(err), "could not delete %s release to %s/%s", tag, owner, repo)
	}

	fmt.Fprintf(os.Stdout, "Deleted %s release\n", tag)

	if releaseDeleteWithTag {
		resp, err := client.Git.DeleteRef(ctx, owner, repo, fmt.Sprintf("tags/%s", tag))
		if err != nil {
			return errors.Wrapf(checkRateLimitError(err), "could not delete %s release to %s/%s", tag, owner, repo)
		}
		if resp.StatusCode != http.StatusOK {
			return errors.Wrap(err, "failed")
		}
		fmt.Fprintf(os.Stdout, "Deleted %s tag\n", tag)
	}

	return nil
}
