// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"os"

	"github.com/google/go-github/v21/github"
	"golang.org/x/oauth2"
)

func newClient(ctx context.Context) *github.Client {
	token := os.Getenv("GHCTL_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
		if token == "" {
			return github.NewClient(nil)
		}
	}
	source := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
	})
	return github.NewClient(oauth2.NewClient(ctx, source))
}

func newClientFromToken(ctx context.Context, token string) *github.Client {
	source := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: token,
	})

	return github.NewClient(oauth2.NewClient(ctx, source))
}
