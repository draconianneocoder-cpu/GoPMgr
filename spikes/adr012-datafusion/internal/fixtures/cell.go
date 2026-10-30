// SPDX-FileCopyrightText: 2026 James L. Burns and The GoPMgr Contributors
// SPDX-License-Identifier: GPL-3.0-or-later

package fixtures

// Kind is the logical type of a cell, independent of any engine.
type Kind string

const (
	KNull  Kind = "null"
	KInt   Kind = "int"   // exact integer, base-10 text
	KDec   Kind = "dec"   // exact decimal, base-10 text with optional fraction
	KFloat Kind = "float" // IEEE-754 binary64, strconv 'g' -1 text (NaN, +Inf, -Inf allowed)
	KBool  Kind = "bool"  // "true" or "false"
	KStr   Kind = "str"   // UTF-8 text, exact
	KTime  Kind = "time"  // instant, RFC 3339 in UTC with nanoseconds: 2026-01-02T03:04:05.123456789Z
	KDate  Kind = "date"  // calendar date: 2026-01-02
	KBytes Kind = "bytes" // lowercase hex
	KJSON  Kind = "json"  // nested value as canonical JSON (sorted keys, no spaces)
	KText  Kind = "text"  // a delimited-text field as written; the file defines no type
)

// Cell is one expected value.
type Cell struct {
	K Kind   `json:"k"`
	V string `json:"v"`
}

func T(s string) Cell      { return Cell{KText, s} }
func S(s string) Cell      { return Cell{KStr, s} }
func I(s string) Cell      { return Cell{KInt, s} }
func D(s string) Cell      { return Cell{KDec, s} }
func F(s string) Cell      { return Cell{KFloat, s} }
func B(b bool) Cell        { return Cell{KBool, map[bool]string{true: "true", false: "false"}[b]} }
func N() Cell              { return Cell{KNull, ""} }
func Tm(s string) Cell     { return Cell{KTime, s} }
func Dt(s string) Cell     { return Cell{KDate, s} }
func Hex(s string) Cell    { return Cell{KBytes, s} }
func J(s string) Cell      { return Cell{KJSON, s} }
func row(c ...Cell) []Cell { return c }
