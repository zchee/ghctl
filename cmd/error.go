// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/zchee/ghctl/internal/errors"
)

var (
	ErrRateLimit = errors.New("hit GitHub API rate limit")
)
