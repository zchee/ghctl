// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	spin "github.com/tj/go-spin"
	"github.com/urfave/cli"
)

type checkType int

const (
	exactArgs checkType = iota
	minArgs
	maxArgs
)

func checkArgs(c *cli.Context, expected int, typ checkType, args ...string) error {
	cmdName := c.Command.FullName()
	var err error
	switch typ {
	case exactArgs:
		if c.NArg() != expected {
			err = errors.Errorf("%q command requires exactly %s %d argument(s)", cmdName, strings.Join(args, " "), expected)
		}
	case minArgs:
		if c.NArg() < expected {
			err = errors.Errorf("%q command requires a minimum of %s %d argument(s)", cmdName, strings.Join(args, " "), expected)
		}
	case maxArgs:
		if c.NArg() > expected {
			err = errors.Errorf("%q command requires a maximum of %s %d argument(s)", cmdName, strings.Join(args, " "), expected)
		}
	}

	if err != nil {
		return err
	}
	return nil
}

// Spin represents a loading spinner.
type Spin struct {
	s *spin.Spinner
}

func newSpin() *Spin {
	s := spin.New()
	s.Set(spin.Spin1)
	return &Spin{
		s: s,
	}
}

func (s *Spin) next(desc ...string) {
	fmt.Fprintf(os.Stderr, "\r%s %s %s", color.BlueString(desc[0]), s.s.Next(), strings.Join(desc[1:], " "))
}

func (s *Spin) flush() {
	fmt.Fprint(os.Stderr, "\r\n")
}
