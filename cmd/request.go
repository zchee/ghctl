// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"

	"github.com/google/go-github/v38/github"
)

func getUser(ctx context.Context, client *github.Client) (*github.User, error) {
	user, _, err := client.Users.Get(ctx, "")
	if _, ok := err.(*github.RateLimitError); ok {
		return nil, ErrRateLimit
	}
	return user, nil
}
