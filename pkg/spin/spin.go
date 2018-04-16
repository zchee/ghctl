// Copyright 2018 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package spin

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/fatih/color"
	spin "github.com/tj/go-spin"
)

// Spin represents a loading spinner.
type Spin struct {
	s  *spin.Spinner
	mu sync.Mutex
}

func NewSpin() *Spin {
	s := spin.New()
	s.Set(spin.Spin1)
	return &Spin{
		s: s,
	}
}

func (s *Spin) Next(desc ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Fprintf(os.Stderr, "\r%s %s %s", color.BlueString(desc[0]), s.s.Next(), strings.Join(desc[1:], " "))
}

func (s *Spin) Flush() {
	fmt.Fprint(os.Stderr, "\r")
}
