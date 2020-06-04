// Copyright 2018 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors

import (
	"errors"

	"github.com/google/go-github/v28/github"
)

var (
	ErrRateLimit = errors.New("hit GitHub API rate limit")
)

func IsRateLimitErr(err error) bool {
	if _, ok := err.(*github.RateLimitError); ok {
		return true
	}
	return false
}
