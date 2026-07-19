// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package legex

import (
	"bufio"
	"os"
	"path/filepath"
	"reflect"
	"regexp/syntax"
	"strconv"
	"strings"
	"testing"
)

type upstreamFowlerMatchBlock struct {
	block    string
	captures []string
}

func TestLegex_UpstreamFowlerStreaming(t *testing.T) {
	files, err := filepath.Glob("testdata/*.dat")
	if err != nil {
		t.Fatal(err)
	}
	for _, filename := range files {
		t.Run(filepath.Base(filename), func(t *testing.T) {
			testUpstreamFowlerStreaming(t, filename)
		})
	}
}

func testUpstreamFowlerStreaming(t *testing.T, filename string) {
	file, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	lineNumber := 0
	lastRegexp := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if line == "" || line[0] == '#' {
			continue
		}

		fields := strings.FieldsFunc(line, func(r rune) bool { return r == '\t' })
		skip := false
		for i, field := range fields {
			switch field {
			case "NULL":
				fields[i] = ""
			case "NIL":
				skip = true
			}
		}
		if skip || len(fields) == 0 {
			continue
		}

		flag := fields[0]
		switch flag[0] {
		case '?', '&', '|', ';', '{', '}':
			flag = flag[1:]
			if flag == "" {
				continue
			}
		case ':':
			var ok bool
			if _, flag, ok = strings.Cut(flag[1:], ":"); !ok {
				continue
			}
		case 'C', 'N', 'T', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			continue
		}
		if len(fields) < 4 {
			t.Errorf("%s:%d: too few fields: %s", filename, lineNumber, line)
			continue
		}

		if strings.Contains(flag, "$") {
			quoted := `"` + fields[1] + `"`
			fields[1], err = strconv.Unquote(quoted)
			if err != nil {
				t.Errorf("%s:%d: cannot unquote %s", filename, lineNumber, quoted)
				continue
			}
			quoted = `"` + fields[2] + `"`
			fields[2], err = strconv.Unquote(quoted)
			if err != nil {
				t.Errorf("%s:%d: cannot unquote %s", filename, lineNumber, quoted)
				continue
			}
		}

		if fields[1] == "SAME" {
			fields[1] = lastRegexp
		}
		lastRegexp = fields[1]
		text := fields[2]
		ok, shouldCompile, shouldMatch, positions := parseUpstreamFowlerResult(fields[3])
		if !ok {
			t.Errorf("%s:%d: cannot parse result %q", filename, lineNumber, fields[3])
			continue
		}

		for _, mode := range flag {
			pattern := fields[1]
			flags := syntax.POSIX | syntax.ClassNL
			switch mode {
			case 'E':
			case 'L':
				pattern = QuoteMeta(pattern)
			default:
				continue
			}
			if strings.ContainsRune(flag, 'i') {
				flags |= syntax.FoldCase
			}

			re, compileErr := compile(pattern, flags, true)
			if compileErr != nil {
				if shouldCompile {
					t.Errorf("%s:%d: %q did not compile: %v", filename, lineNumber, pattern, compileErr)
				}
				continue
			}
			if !shouldCompile {
				t.Errorf("%s:%d: %q should not compile", filename, lineNumber, pattern)
				continue
			}

			havePositions := upstreamStreamFirstSubmatchIndex(re, text)
			if (havePositions != nil) != shouldMatch {
				t.Errorf("%s:%d: %q match on %q = %v, want %v", filename, lineNumber, pattern, text, havePositions != nil, shouldMatch)
				continue
			}
			if len(positions) == 0 {
				continue
			}
			if len(havePositions) > len(positions) {
				havePositions = havePositions[:len(positions)]
			}
			have := upstreamFowlerMatchBlocks(text, havePositions)
			want := upstreamFowlerMatchBlocks(text, positions)
			if !reflect.DeepEqual(have, want) {
				t.Errorf("%s:%d: %q on %q = %v, want %v", filename, lineNumber, pattern, text, have, want)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func upstreamFowlerMatchBlocks(input string, positions []int) []upstreamFowlerMatchBlock {
	if positions == nil {
		return nil
	}
	block := upstreamFowlerMatchBlock{block: input[positions[0]:positions[1]]}
	for i := 2; i < len(positions); i += 2 {
		capture := ""
		if positions[i] >= 0 && positions[i+1] >= 0 {
			capture = input[positions[i]:positions[i+1]]
		}
		block.captures = append(block.captures, capture)
	}
	return []upstreamFowlerMatchBlock{block}
}

func parseUpstreamFowlerResult(result string) (ok, compiled, matched bool, positions []int) {
	switch {
	case result == "":
		return true, true, true, nil
	case result == "NOMATCH":
		return true, true, false, nil
	case 'A' <= result[0] && result[0] <= 'Z':
		return true, false, false, nil
	}
	compiled = true

	for result != "" {
		end := byte(')')
		if len(positions)%2 == 0 {
			if result[0] != '(' {
				return false, false, false, nil
			}
			result = result[1:]
			end = ','
		}
		i := strings.IndexByte(result, end)
		if i <= 0 {
			return false, false, false, nil
		}
		value := -1
		if result[:i] != "?" {
			var err error
			value, err = strconv.Atoi(result[:i])
			if err != nil {
				return false, false, false, nil
			}
		}
		positions = append(positions, value)
		result = result[i+1:]
	}
	if len(positions)%2 != 0 {
		return false, false, false, nil
	}
	return true, true, true, positions
}

func upstreamStreamFirstSubmatchIndex(re *Regexp, input string) []int {
	machine := re.Get()
	defer machine.Close()
	ctx := NewStreamContext()
	retained := make([]byte, 0, len(input))
	released := 0

	for i := range len(input) {
		retained = append(retained, input[i])
		index, _, ok := machine.Match(ctx, retained)
		if ok {
			return upstreamGlobalCaptures(machine.MatchCap(), released)
		}
		ctx = ctx.Advance(retained[:index])
		retained = retained[index:]
		released += index
	}
	if _, _, ok := machine.Finish(ctx, retained); !ok {
		return nil
	}
	return upstreamGlobalCaptures(machine.MatchCap(), released)
}

func upstreamGlobalCaptures(captures []int, released int) []int {
	for i, position := range captures {
		if position >= 0 {
			captures[i] = position + released
		}
	}
	return captures
}
