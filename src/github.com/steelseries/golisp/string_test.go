// Copyright 2013 David R. Astels. All rights reserved.

//This package impliments a basic LISP interpretor for embedding in a go program for scripting.
// This file inmpliments tests for numeric atoms
package golisp

import (
    . "launchpad.net/gocheck"
)

type StringAtomSuite struct {
    atom *Data
}

var _ = Suite(&StringAtomSuite{})

func (s *StringAtomSuite) SetUpTest(c *C) {
    s.atom = StringWithValue("Hello, world.")
}

func (s *StringAtomSuite) TestNumericValue(c *C) {
    c.Assert(NumericValue(s.atom), Equals, uint32(0))
}

func (s *StringAtomSuite) TestString(c *C) {
    c.Assert(String(s.atom), Equals, `"Hello, world."`)
}

func (s *StringAtomSuite) TestEval(c *C) {
    d, err := Eval(s.atom, Global)
    c.Assert(err, IsNil)
    c.Assert(d, Equals, s.atom)
}

func (s *StringAtomSuite) TestBooleanValue(c *C) {
    c.Assert(BooleanValue(s.atom), Equals, true)
}