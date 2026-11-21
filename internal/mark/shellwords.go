// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package mark

import (
	"errors"
	"strings"
)

// SplitCommand splits a command line into arguments using POSIX shell quoting:
// single quotes keep everything literal, double quotes allow backslash escapes
// of `"`, `\`, `$` and backtick, and a backslash outside quotes escapes the
// next character. It does no expansion of variables, globs, or pipes; the
// command runs directly, never through a shell.
func SplitCommand(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inArg := false
	const (
		none = iota
		single
		double
	)
	quote := none
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch quote {
		case single:
			if c == '\'' {
				quote = none
			} else {
				cur.WriteByte(c)
			}
			continue
		case double:
			switch {
			case c == '"':
				quote = none
			case c == '\\' && i+1 < len(s) && strings.ContainsRune("\"\\$`", rune(s[i+1])):
				i++
				cur.WriteByte(s[i])
			default:
				cur.WriteByte(c)
			}
			continue
		}
		switch c {
		case ' ', '\t', '\n':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		case '\'':
			quote, inArg = single, true
		case '"':
			quote, inArg = double, true
		case '\\':
			if i+1 < len(s) {
				i++
				cur.WriteByte(s[i])
			}
			inArg = true
		default:
			cur.WriteByte(c)
			inArg = true
		}
	}
	if quote != none {
		return nil, errors.New("llm command has an unterminated quote")
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args, nil
}
