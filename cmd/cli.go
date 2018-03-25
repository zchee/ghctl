// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/fatih/color"
	cli "github.com/spf13/cobra"
	spin "github.com/tj/go-spin"
	"github.com/pkg/errors"
)

type checkType int

const (
	exactArgs checkType = iota
	minArgs
	maxArgs
)

func checkArgs(cmd *cli.Command, args []string, expected int, typ checkType, value ...string) error {
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

// Spin represents a loading spinner.
type Spin struct {
	s  *spin.Spinner
	mu sync.Mutex
}

func newSpin() *Spin {
	s := spin.New()
	s.Set(spin.Spin1)
	return &Spin{
		s: s,
	}
}

func (s *Spin) next(desc ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(os.Stderr, "\r%s %s %s", color.BlueString(desc[0]), s.s.Next(), strings.Join(desc[1:], " "))
}

func (s *Spin) flush() {
	fmt.Fprint(os.Stderr, "\r")
}
