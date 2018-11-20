// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type checkType int

const (
	exactArgs checkType = iota
	minArgs
	maxArgs
)

func checkArgs(cmd *cobra.Command, args []string, expected int, typ checkType, value ...string) error {
	cmdName := cmd.Name()
	var err error
	switch typ {
	case exactArgs:
		if len(args) != expected {
			err = errors.Errorf("%q command requires exactly %s %d argument(s)", cmdName, strings.Join(value, " "), expected)
		}
	case minArgs:
		if len(args) < expected {
			err = errors.Errorf("%q command requires a minimum of %s %d argument(s)", cmdName, strings.Join(value, " "), expected)
		}
	case maxArgs:
		if len(args) > expected {
			err = errors.Errorf("%q command requires a maximum of %s %d argument(s)", cmdName, strings.Join(value, " "), expected)
		}
	}

	if err != nil {
		return err
	}
	return nil
}
