// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
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

type Spin struct {
	s *spin.Spinner
}

func NewSpin() *Spin {
	s := spin.New()
	s.Set(spin.Spin1)
	return &Spin{
		s: s,
	}
}

func (s *Spin) Next(desc ...string) {
	firstDesc := desc[0]
	fmt.Printf("\r%s %s %s", color.BlueString(firstDesc), s.s.Next(), strings.Join(desc[1:], ""))
}
