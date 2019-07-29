// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/google/go-github/v24/github"
	"golang.org/x/xerrors"
)

var (
	ErrRateLimit = xerrors.New("hit GitHub API rate limit")
)

func checkRateLimitError(err error) error {
	if _, ok := err.(*github.RateLimitError); ok {
		return ErrRateLimit
	}
	return err
}

var (
	errRateLimit    *github.RateLimitError
	errmsgRateLimit = xerrors.New("hit GitHub API rate limit")
)

func IsRateLimitError(err error) error {
	if xerrors.As(err, &errRateLimit) {
		return errRateLimit
	}

	return err
}
