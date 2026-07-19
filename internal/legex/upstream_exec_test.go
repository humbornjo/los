// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package legex_test

import (
	"bufio"
	"compress/bzip2"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/humbornjo/los/internal/legex"
)

func TestLegex_UpstreamRE2SearchStreaming(t *testing.T) {
	testUpstreamRE2Streaming(t, "testdata/re2-search.txt")
}

func testUpstreamRE2Streaming(t *testing.T, filename string) {
	file, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var input io.Reader = file
	if strings.HasSuffix(filename, ".bz2") {
		input = bzip2.NewReader(file)
		filename = strings.TrimSuffix(filename, ".bz2")
	}

	var (
		texts     []string
		remaining []string
		inStrings bool
		machines  [4]*legex.Regexp
		failures  int
		cases     int
	)
	scanner := bufio.NewScanner(input)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		switch {
		case line == "":
			t.Fatalf("%s:%d: unexpected blank line", filename, lineNumber)
		case line[0] == '#':
			continue
		case 'A' <= line[0] && line[0] <= 'Z':
			t.Log(line)
			continue
		case line == "strings":
			texts = texts[:0]
			inStrings = true
		case line == "regexps":
			inStrings = false
		case line[0] == '"':
			value, err := strconv.Unquote(line)
			if err != nil {
				t.Fatalf("%s:%d: unquote %s: %v", filename, lineNumber, line, err)
			}
			if inStrings {
				texts = append(texts, value)
				continue
			}
			if len(remaining) != 0 {
				t.Fatalf("%s:%d: %d strings remain before %q", filename, lineNumber, len(remaining), value)
			}
			if strings.Contains(value, `\C`) {
				machines = [4]*legex.Regexp{}
				continue
			}

			full := `\A(?:` + value + `)\z`
			machines[0], err = legex.Compile(full)
			if err == nil {
				machines[1], err = legex.Compile(value)
			}
			if err == nil {
				machines[2], err = legex.Compile(full)
			}
			if err == nil {
				machines[3], err = legex.Compile(value)
			}
			if err != nil {
				t.Errorf("%s:%d: compile %q: %v", filename, lineNumber, value, err)
				failures++
				machines = [4]*legex.Regexp{}
				continue
			}
			machines[2].Longest()
			machines[3].Longest()
			remaining = texts
		case line[0] == '-' || '0' <= line[0] && line[0] <= '9':
			cases++
			if machines[0] == nil {
				continue
			}
			if len(remaining) == 0 {
				t.Fatalf("%s:%d: no input remains", filename, lineNumber)
			}
			text := remaining[0]
			remaining = remaining[1:]
			if !upstreamSingleByteRunes(text) && strings.Contains(machines[1].String(), `\B`) {
				continue
			}

			results := strings.Split(line, ";")
			if len(results) != len(machines) {
				t.Fatalf("%s:%d: got %d results, want %d", filename, lineNumber, len(results), len(machines))
			}
			for i, result := range results {
				want := upstreamSingleMatchBlocks(text, parseUpstreamRE2Result(t, filename, lineNumber, result))
				have := streamFirstMatchBlocks(machines[i], text)
				if !reflect.DeepEqual(have, want) {
					t.Errorf("%s:%d: mode=%d %q on %q = %v, want %v", filename, lineNumber, i, machines[i], text, have, want)
					failures++
					if failures >= 100 {
						t.Fatalf("stopping after %d failures", failures)
					}
				}
			}
		default:
			t.Fatalf("%s:%d: out of sync: %s", filename, lineNumber, line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("%s:%d: %v", filename, lineNumber, err)
	}
	if len(remaining) != 0 {
		t.Fatalf("%s:%d: %d strings remain at EOF", filename, lineNumber, len(remaining))
	}
	t.Logf("%d cases tested", cases)
}

func upstreamSingleByteRunes(s string) bool {
	for _, r := range s {
		if r >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func parseUpstreamRE2Result(t *testing.T, filename string, lineNumber int, result string) []int {
	t.Helper()
	if result == "-" {
		return nil
	}

	fields := strings.Fields(result)
	positions := make([]int, 0, 2*len(fields))
	for _, field := range fields {
		if field == "-" {
			positions = append(positions, -1, -1)
			continue
		}
		startText, endText, ok := strings.Cut(field, "-")
		start, startErr := strconv.Atoi(startText)
		end, endErr := strconv.Atoi(endText)
		if !ok || startErr != nil || endErr != nil || start > end {
			t.Fatalf("%s:%d: invalid result pair %q", filename, lineNumber, field)
		}
		positions = append(positions, start, end)
	}
	return positions
}

func TestLegex_UpstreamLongestStreaming(t *testing.T) {
	expr, input := `a(|b)`, "ab"
	first := legex.MustCompile(expr)
	longest := legex.MustCompile(expr)
	longest.Longest()
	firstExpected := []upstreamMatchBlock{{block: "a", captures: []string{""}}}
	if got := streamFirstMatchBlocks(first, input); !reflect.DeepEqual(got, firstExpected) {
		t.Fatalf("first match = %v", got)
	}
	longestExpected := []upstreamMatchBlock{{block: "ab", captures: []string{"b"}}}
	if got := streamFirstMatchBlocks(longest, input); !reflect.DeepEqual(got, longestExpected) {
		t.Fatalf("longest match = %v", got)
	}
}

func TestLegex_UpstreamProgramTooLongStreaming(t *testing.T) {
	expr := `(one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|twenty|twentyone|twentytwo|twentythree|twentyfour|twentyfive|twentysix|twentyseven|twentyeight|twentynine|thirty|thirtyone|thirtytwo|thirtythree|thirtyfour|thirtyfive|thirtysix|thirtyseven|thirtyeight|thirtynine|forty|fortyone|fortytwo|fortythree|fortyfour|fortyfive|fortysix|fortyseven|fortyeight|fortynine|fifty|fiftyone|fiftytwo|fiftythree|fiftyfour|fiftyfive|fiftysix|fiftyseven|fiftyeight|fiftynine|sixty|sixtyone|sixtytwo|sixtythree|sixtyfour|sixtyfive|sixtysix|sixtyseven|sixtyeight|sixtynine|seventy|seventyone|seventytwo|seventythree|seventyfour|seventyfive|seventysix|seventyseven|seventyeight|seventynine|eighty|eightyone|eightytwo|eightythree|eightyfour|eightyfive|eightysix|eightyseven|eightyeight|eightynine|ninety|ninetyone|ninetytwo|ninetythree|ninetyfour|ninetyfive|ninetysix|ninetyseven|ninetyeight|ninetynine|onehundred)`
	re := legex.MustCompile(expr)
	expected := []upstreamMatchBlock{{block: "two", captures: []string{"two"}}}
	if got := streamFirstMatchBlocks(re, "two"); !reflect.DeepEqual(got, expected) {
		t.Fatalf("match = %v", got)
	}
	if got := streamFirstMatchBlocks(re, "xxx"); got != nil {
		t.Fatalf("unexpected match = %v", got)
	}
}
