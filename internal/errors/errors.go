// Copyright 2017 The ghctl Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors

import (
	"runtime"

	"github.com/rhysd/locerr"
)

func init() {
	locerr.SetColor(true)
}

func New(msg string) error {
	return locerr.NewError(msg)
}

func Errorf(format string, a ...interface{}) error {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		locerr.Errorf(format, a...)
	}
	pos := locerr.Pos{
		File: &locerr.Source{
			Path:   file,
			Exists: true,
		},
		Line: line,
	}
	return locerr.ErrorfAt(pos, format, a...)
}

func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		locerr.Note(err, msg)
	}
	pos := locerr.Pos{
		File: &locerr.Source{
			Path:   file,
			Exists: true,
		},
		Line: line,
	}
	return locerr.NoteAt(pos, err, msg)
}

func Wrapf(err error, format string, a ...interface{}) error {
	if err == nil {
		return nil
	}
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		locerr.Notef(err, format, a...)
	}
	pos := locerr.Pos{
		File: &locerr.Source{
			Path:   file,
			Exists: true,
		},
		Line: line,
	}
	return locerr.NotefAt(pos, err, format, a...)
}

func WithStack(err error) error {
	if err == nil {
		return nil
	}
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		locerr.NewError(err.Error())
	}
	pos := locerr.Pos{
		File: &locerr.Source{
			Path:   file,
			Exists: true,
		},
		Line: line,
	}
	return locerr.WithPos(pos, err)
}
