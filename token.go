// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"

	"github.com/google/go-github/github"
	"golang.org/x/net/context"
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
