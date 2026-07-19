# Go regexp test coverage

This directory ports the direct Go 1.26.5 `/usr/local/go/src/regexp/*_test.go`
suite to LOS's ordered streaming NFA. Matching assertions feed input one byte at
a time and compare a list of matched blocks plus capture strings. Empty matches
remain valid at the internal `legex.Machine` layer even though the public LOS
head/tail matcher rejects them because it must always make progress.

The RE2 and Fowler fixtures are vendored in `testdata` with their upstream
license. Replacement, split, expansion, reader, and one-pass/backtracking APIs
that LOS does not expose are projected onto their observable regex matching
operation. Pure backend helpers with no regex/input behavior are explicitly
marked not applicable.

| Upstream function | LOS coverage | Interpretation |
| --- | --- | --- |
| `all_test.go:TestGoodCompile` | `TestLegex_UpstreamCompileTables` | All 17 patterns compile and run as stream cases. |
| `all_test.go:TestBadCompile` | `TestLegex_UpstreamCompileTables` | All 12 compile failures and messages. |
| `all_test.go:TestMatch` | `TestLegex_UpstreamFindCasesStreaming` | All 87 find cases as streamed match-block lists. |
| `all_test.go:TestMatchFunction` | `TestLegex_UpstreamFindCasesStreaming` | Package helper has the same matching cases. |
| `all_test.go:TestCopyMatch` | `TestLegex_UpstreamFindCasesStreaming` | Repeated machines from one immutable compiled regexp. |
| `all_test.go:TestReplaceAll` | `TestLegex_UpstreamAllSemanticTablesStreaming` | All 72 pattern/input rows; replacement is not a LOS API. |
| `all_test.go:TestReplaceAllLiteral` | `TestLegex_UpstreamAllSemanticTablesStreaming` | All 15 literal-replacement pattern/input rows. |
| `all_test.go:TestReplaceAllFunc` | `TestLegex_UpstreamAllSemanticTablesStreaming` | All 3 function-replacement pattern/input rows. |
| `all_test.go:TestQuoteMeta` | `TestLegex_UpstreamQuoteMetaAndLiteralPrefix` | Exact quoting plus streamed literal matches. |
| `all_test.go:TestLiteralPrefix` | `TestLegex_UpstreamQuoteMetaAndLiteralPrefix` | All 17 metadata and anchored-prefix rows. |
| `all_test.go:TestSubexp` | `TestLegex_UpstreamSubexpMetadata` | All 11 name/index rows; captures are exercised by stream cases. |
| `all_test.go:TestSplit` | `TestLegex_UpstreamAllSemanticTablesStreaming` | All 23 pattern/input rows; splitting is not a LOS API. |
| `all_test.go:TestParseAndCompile` | `TestLegex_UpstreamParseAndCompileStreaming` | Both syntax-flag cases streamed. |
| `all_test.go:TestOnePassCutoff` | `TestLegex_UpstreamAllSemanticTablesStreaming` | Exact cutoff pattern run by the ordered NFA. |
| `all_test.go:TestSwitchBacktrack` | `TestLegex_UpstreamAllSemanticTablesStreaming` | Long and short inputs run through pooled NFA machines. |
| `all_test.go:TestDeepEqual` | `TestLegex_UpstreamDeepEqual` | Immutable regexp equality before/after a streamed match. |
| `all_test.go:TestMinInputLen` | `TestLegex_UpstreamAllSemanticTablesStreaming` | All 12 patterns with minimum-length witnesses; internal optimization is absent. |
| `all_test.go:TestUnmarshalText` | `TestLegex_UpstreamUnmarshalText` | All good patterns round-trip and then stream-match. |
| `exec_test.go:TestRE2Search` | `TestLegex_UpstreamRE2SearchStreaming` | All 1,888 fixture rows in full/partial and first/longest modes. |
| `exec_test.go:TestFowler` | `TestLegex_UpstreamFowlerStreaming` | Every supported ERE/literal row in all three `.dat` files. |
| `exec_test.go:TestLongest` | `TestLegex_UpstreamLongestStreaming` | First and longest match blocks and captures. |
| `exec_test.go:TestProgramTooLongForBacktrack` | `TestLegex_UpstreamProgramTooLongStreaming` | Exact 100-alternative pattern, match and non-match. |
| `exec2_test.go:TestRE2Exhaustive` | `TestLegex_UpstreamRE2ExhaustiveStreaming` | All 5,716,884 fixture rows; same `!race` and short-mode guards. |
| `find_test.go:TestFind` | `TestLegex_UpstreamFindCasesStreaming` | First block is included in the full streamed block list. |
| `find_test.go:TestFindString` | `TestLegex_UpstreamFindCasesStreaming` | String matching is the streamed byte input. |
| `find_test.go:TestFindIndex` | `TestLegex_UpstreamFindCasesStreaming` | Indices are rendered as matched blocks. |
| `find_test.go:TestFindStringIndex` | `TestLegex_UpstreamFindCasesStreaming` | Indices are rendered as matched blocks. |
| `find_test.go:TestFindReaderIndex` | `TestLegex_UpstreamFindCasesStreaming` | Reader input is interpreted as byte-sized chunks. |
| `find_test.go:TestFindAll` | `TestLegex_UpstreamFindCasesStreaming` | Full ordered block list. |
| `find_test.go:TestFindAllString` | `TestLegex_UpstreamFindCasesStreaming` | Full ordered block list. |
| `find_test.go:TestFindAllIndex` | `TestLegex_UpstreamFindCasesStreaming` | Full ordered block list. |
| `find_test.go:TestFindAllStringIndex` | `TestLegex_UpstreamFindCasesStreaming` | Full ordered block list. |
| `find_test.go:TestFindSubmatch` | `TestLegex_UpstreamFindCasesStreaming` | Capture strings accompany each block. |
| `find_test.go:TestFindStringSubmatch` | `TestLegex_UpstreamFindCasesStreaming` | Capture strings accompany each block. |
| `find_test.go:TestFindSubmatchIndex` | `TestLegex_UpstreamFindCasesStreaming` | Capture indices are rendered as strings. |
| `find_test.go:TestFindStringSubmatchIndex` | `TestLegex_UpstreamFindCasesStreaming` | Capture indices are rendered as strings. |
| `find_test.go:TestFindReaderSubmatchIndex` | `TestLegex_UpstreamFindCasesStreaming` | Reader input is byte-sized chunks with capture strings. |
| `find_test.go:TestFindAllSubmatch` | `TestLegex_UpstreamFindCasesStreaming` | Full block and capture list. |
| `find_test.go:TestFindAllStringSubmatch` | `TestLegex_UpstreamFindCasesStreaming` | Full block and capture list. |
| `find_test.go:TestFindAllSubmatchIndex` | `TestLegex_UpstreamFindCasesStreaming` | Full block and capture list. |
| `find_test.go:TestFindAllStringSubmatchIndex` | `TestLegex_UpstreamFindCasesStreaming` | Full block and capture list. |
| `onepass_test.go:TestMergeRuneSet` | Not applicable | Private one-pass rune-table helper; no regex or input case exists. |
| `onepass_test.go:TestCompileOnePass` | `TestLegex_UpstreamOnePassPatternsStreaming` | All 37 patterns run over a generated streamed corpus; backend classification is absent. |
| `onepass_test.go:TestRunOnePass` | `TestLegex_UpstreamRunOnePassCaseStreaming` | Exact pattern/input with capture strings. |
| `example_test.go:Example` | `TestLegex_UpstreamExamplePatternsStreaming` | All four identifier inputs. |
| `example_test.go:ExampleMatch` | `TestLegex_UpstreamExamplePatternsStreaming`, `TestLegex_UpstreamExampleCompileAndMetadata` | Both matches and invalid compile. |
| `example_test.go:ExampleMatchString` | Same as `ExampleMatch` | String input is streamed as bytes. |
| `example_test.go:ExampleQuoteMeta` | `TestLegex_UpstreamExampleCompileAndMetadata` | Exact quoted output. |
| `example_test.go:ExampleRegexp_Find` | `TestLegex_UpstreamExamplePatternsStreaming` | Streamed block list. |
| `example_test.go:ExampleRegexp_FindAll` | `TestLegex_UpstreamExamplePatternsStreaming` | Streamed block list. |
| `example_test.go:ExampleRegexp_FindAllSubmatch` | `TestLegex_UpstreamExamplePatternsStreaming` | Blocks plus captures. |
| `example_test.go:ExampleRegexp_FindSubmatch` | `TestLegex_UpstreamExamplePatternsStreaming` | Blocks plus captures. |
| `example_test.go:ExampleRegexp_Match` | `TestLegex_UpstreamExamplePatternsStreaming` | Matching and non-matching inputs. |
| `example_test.go:ExampleRegexp_FindString` | `TestLegex_UpstreamExamplePatternsStreaming` | Matching and non-matching inputs. |
| `example_test.go:ExampleRegexp_FindStringIndex` | `TestLegex_UpstreamExamplePatternsStreaming` | Blocks replace index output. |
| `example_test.go:ExampleRegexp_FindStringSubmatch` | `TestLegex_UpstreamExamplePatternsStreaming` | Blocks plus captures. |
| `example_test.go:ExampleRegexp_FindAllString` | `TestLegex_UpstreamExamplePatternsStreaming` | All four calls, including the limited-call duplicate. |
| `example_test.go:ExampleRegexp_FindAllStringSubmatch` | `TestLegex_UpstreamExamplePatternsStreaming` | All four block/capture lists. |
| `example_test.go:ExampleRegexp_FindAllStringSubmatchIndex` | `TestLegex_UpstreamExamplePatternsStreaming` | All five block/capture lists. |
| `example_test.go:ExampleRegexp_FindSubmatchIndex` | `TestLegex_UpstreamExamplePatternsStreaming` | All five first-match inputs. |
| `example_test.go:ExampleRegexp_Longest` | `TestLegex_UpstreamLongestStreaming` | First and longest block/capture lists. |
| `example_test.go:ExampleRegexp_MatchString` | `TestLegex_UpstreamExamplePatternsStreaming` | All three inputs. |
| `example_test.go:ExampleRegexp_NumSubexp` | `TestLegex_UpstreamExampleCompileAndMetadata` | Both metadata calls. |
| `example_test.go:ExampleRegexp_ReplaceAll` | `TestLegex_UpstreamExamplePatternsStreaming` | Both patterns projected to block/capture lists. |
| `example_test.go:ExampleRegexp_ReplaceAllLiteralString` | `TestLegex_UpstreamExamplePatternsStreaming` | Pattern/input projected to matching. |
| `example_test.go:ExampleRegexp_ReplaceAllString` | `TestLegex_UpstreamExamplePatternsStreaming` | Both patterns projected to matching. |
| `example_test.go:ExampleRegexp_ReplaceAllStringFunc` | `TestLegex_UpstreamExamplePatternsStreaming` | Pattern/input projected to matching. |
| `example_test.go:ExampleRegexp_SubexpNames` | `TestLegex_UpstreamExamplePatternsStreaming`, `TestLegex_UpstreamExampleCompileAndMetadata` | Match captures and names. |
| `example_test.go:ExampleRegexp_SubexpIndex` | Same as `SubexpNames` | Match captures and index. |
| `example_test.go:ExampleRegexp_Split` | `TestLegex_UpstreamExamplePatternsStreaming` | Both patterns projected to match blocks. |
| `example_test.go:ExampleRegexp_Expand` | `TestLegex_UpstreamExamplePatternsStreaming` | Multiline blocks and named captures; expansion is not a LOS API. |
| `example_test.go:ExampleRegexp_ExpandString` | `TestLegex_UpstreamExamplePatternsStreaming` | String form of the same streamed capture case. |
| `example_test.go:ExampleRegexp_FindIndex` | `TestLegex_UpstreamExamplePatternsStreaming` | First multiline block and captures. |
| `example_test.go:ExampleRegexp_FindAllSubmatchIndex` | `TestLegex_UpstreamExamplePatternsStreaming` | All multiline blocks and captures. |
| `example_test.go:ExampleRegexp_FindAllIndex` | `TestLegex_UpstreamExamplePatternsStreaming` | Both calls represented by the full block list. |
