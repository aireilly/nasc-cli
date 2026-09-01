// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package parse

import "bytes"

// SplitFrontmatter locates a YAML frontmatter block delimited by --- fences.
// It returns the raw frontmatter bytes (between the fences), the byte offset
// where the body begins, the offsets of the whole block including both fences,
// whether a block was found, and any warning.
//
// A UTF-8 BOM and CRLF are tolerated. An opening fence that never closes is a
// warning, and the function reports ok=false so the whole file is treated as
// body.
func SplitFrontmatter(data []byte) (fm []byte, bodyStart, fmStart, fmEnd int, ok bool, warn string) {
	b := data
	off := 0
	if bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) {
		b = b[3:]
		off = 3
	}
	// Reject TOML / JSON frontmatter with a clear warning.
	if bytes.HasPrefix(b, []byte("+++")) {
		return nil, 0, 0, 0, false, "TOML frontmatter is not supported in v0.1"
	}
	trimmed := bytes.TrimLeft(b, " \t")
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return nil, 0, 0, 0, false, "JSON frontmatter is not supported in v0.1"
	}

	// The opening fence must be the first line: "---" then a newline.
	if !bytes.HasPrefix(b, []byte("---\n")) && !bytes.HasPrefix(b, []byte("---\r\n")) {
		return nil, 0, 0, 0, false, ""
	}
	openLen := 4
	if bytes.HasPrefix(b, []byte("---\r\n")) {
		openLen = 5
	}
	rest := b[openLen:]
	// Find a closing fence on its own line.
	idx := findClosingFence(rest)
	if idx.contentEnd < 0 {
		return nil, 0, 0, 0, false, "frontmatter opening fence has no closing fence; treating file as body"
	}
	fm = rest[:idx.contentEnd]
	fmStart = off
	bodyStart = off + openLen + idx.afterFence
	fmEnd = bodyStart
	return fm, bodyStart, fmStart, fmEnd, true, ""
}

type fenceHit struct {
	contentEnd int // end of frontmatter content (before the closing fence)
	afterFence int // offset in rest just past the closing fence line
}

// findClosingFence scans line by line for a line equal to "---".
func findClosingFence(rest []byte) fenceHit {
	pos := 0
	for pos <= len(rest) {
		nl := bytes.IndexByte(rest[pos:], '\n')
		var line []byte
		var lineEnd int
		if nl < 0 {
			line = rest[pos:]
			lineEnd = len(rest)
		} else {
			line = rest[pos : pos+nl]
			lineEnd = pos + nl + 1
		}
		if isFence(line) {
			return fenceHit{contentEnd: pos, afterFence: lineEnd}
		}
		if nl < 0 {
			break
		}
		pos = lineEnd
	}
	return fenceHit{contentEnd: -1, afterFence: -1}
}

func isFence(line []byte) bool {
	return bytes.Equal(bytes.TrimRight(line, "\r"), []byte("---"))
}
