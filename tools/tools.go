// Copyright 2021 The ghctl Authors
// SPDX-License-Identifier: BSD-3-Clause

//go:build tools

// Package tools manages tools using during development.
package tools

import (
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "golang.org/x/tools/cmd/goimports"
	_ "gotest.tools/gotestsum"
	_ "mvdan.cc/gofumpt"
)
