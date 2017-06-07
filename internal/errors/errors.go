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
	pos := caller()
	if pos == nil {
		return locerr.Errorf(format, a...)
	}
	return locerr.ErrorfAt(*pos, format, a...)
}

func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	pos := caller()
	if pos == nil {
		return locerr.Note(err, msg)
	}
	return locerr.NoteAt(*pos, err, msg)
}

func Wrapf(err error, format string, a ...interface{}) error {
	if err == nil {
		return nil
	}
	pos := caller()
	if pos == nil {
		return locerr.Notef(err, format, a...)
	}
	return locerr.NotefAt(*pos, err, format, a...)
}

func caller() *locerr.Pos {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return nil
	}
	f, err := locerr.NewSourceFromFile(file)
	if err != nil {
		return nil
	}
	return &locerr.Pos{
		File: f,
		Line: line,
	}
}
