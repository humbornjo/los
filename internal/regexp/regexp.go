package regexp

import (
	"bytes"
	"io"
	"regexp/syntax"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// Regexp is the representation of a compiled regular expression.
// A Regexp is safe for concurrent use by multiple goroutines,
// except for configuration methods, such as [Regexp.Longest].
type Regexp struct {
	expr           string       // as passed to Compile
	prog           *syntax.Prog // compiled program
	onepass        *onePassProg // onepass program or nil
	numSubexp      int
	maxBitStateLen int
	subexpNames    []string
	prefix         string         // required prefix in unanchored matches
	prefixBytes    []byte         // prefix, as a []byte
	prefixRune     rune           // first rune in prefix
	prefixEnd      uint32         // pc for last rune in prefix
	mpool          int            // pool for machines
	matchcap       int            // size of recorded match lengths
	prefixComplete bool           // prefix is the entire regexp
	cond           syntax.EmptyOp // empty-width conditions required at start of match
	minInputLen    int            // minimum length of the input in bytes

	// This field can be modified by the Longest method,
	// but it is otherwise read-only.
	longest bool // whether regexp prefers leftmost-longest match
}

// String returns the source text used to compile the regular expression.
func (re *Regexp) String() string {
	return re.expr
}

// Copy returns a new [Regexp] object copied from re.
// Calling [Regexp.Longest] on one copy does not affect another.
//
// Deprecated: In earlier releases, when using a [Regexp] in multiple goroutines,
// giving each goroutine its own copy helped to avoid lock contention.
// As of Go 1.12, using Copy is no longer necessary to avoid lock contention.
// Copy may still be appropriate if the reason for its use is to make
// two copies with different [Regexp.Longest] settings.
func (re *Regexp) Copy() *Regexp {
	re2 := *re
	return &re2
}

// Compile parses a regular expression and returns, if successful,
// a [Regexp] object that can be used to match against text.
//
// When matching against text, the regexp returns a match that
// begins as early as possible in the input (leftmost), and among those
// it chooses the one that a backtracking search would have found first.
// This so-called leftmost-first matching is the same semantics
// that Perl, Python, and other implementations use, although this
// package implements it without the expense of backtracking.
// For POSIX leftmost-longest matching, see [CompilePOSIX].
func Compile(expr string) (*Regexp, error) {
	return compile(expr, syntax.Perl, false)
}

// CompilePOSIX is like [Compile] but restricts the regular expression
// to POSIX ERE (egrep) syntax and changes the match semantics to
// leftmost-longest.
//
// That is, when matching against text, the regexp returns a match that
// begins as early as possible in the input (leftmost), and among those
// it chooses a match that is as long as possible.
// This so-called leftmost-longest matching is the same semantics
// that early regular expression implementations used and that POSIX
// specifies.
//
// However, there can be multiple leftmost-longest matches, with different
// submatch choices, and here this package diverges from POSIX.
// Among the possible leftmost-longest matches, this package chooses
// the one that a backtracking search would have found first, while POSIX
// specifies that the match be chosen to maximize the length of the first
// subexpression, then the second, and so on from left to right.
// The POSIX rule is computationally prohibitive and not even well-defined.
// See https://swtch.com/~rsc/regexp/regexp2.html#posix for details.
func CompilePOSIX(expr string) (*Regexp, error) {
	return compile(expr, syntax.POSIX, true)
}

// Longest makes future searches prefer the leftmost-longest match.
// That is, when matching against text, the regexp returns a match that
// begins as early as possible in the input (leftmost), and among those
// it chooses a match that is as long as possible.
// This method modifies the [Regexp] and may not be called concurrently
// with any other methods.
func (re *Regexp) Longest() {
	re.longest = true
}

func compile(expr string, mode syntax.Flags, longest bool) (*Regexp, error) {
	re, err := syntax.Parse(expr, mode)
	if err != nil {
		return nil, err
	}
	maxCap := re.MaxCap()
	capNames := re.CapNames()

	re = re.Simplify()
	prog, err := syntax.Compile(re)
	if err != nil {
		return nil, err
	}
	matchcap := prog.NumCap
	if matchcap < 2 {
		matchcap = 2
	}
	regexp := &Regexp{
		expr:        expr,
		prog:        prog,
		onepass:     compileOnePass(prog),
		numSubexp:   maxCap,
		subexpNames: capNames,
		cond:        prog.StartCond(),
		longest:     longest,
		matchcap:    matchcap,
		minInputLen: minInputLen(re),
	}
	if regexp.onepass == nil {
		regexp.prefix, regexp.prefixComplete = prog.Prefix()
		regexp.maxBitStateLen = maxBitStateLen(prog)
	} else {
		regexp.prefix, regexp.prefixComplete, regexp.prefixEnd = onePassPrefix(prog)
	}
	if regexp.prefix != "" {
		// TODO(rsc): Remove this allocation by adding
		// IndexString to package bytes.
		regexp.prefixBytes = []byte(regexp.prefix)
		regexp.prefixRune, _ = utf8.DecodeRuneInString(regexp.prefix)
	}

	n := len(prog.Inst)
	i := 0
	for matchSize[i] != 0 && matchSize[i] < n {
		i++
	}
	regexp.mpool = i

	return regexp, nil
}

// Pools of *machine for use during (*Regexp).doExecute,
// split up by the size of the execution queues.
// matchPool[i] machines have queue size matchSize[i].
// On a 64-bit system each queue entry is 16 bytes,
// so matchPool[0] has 16*2*128 = 4kB queues, etc.
// The final matchPool is a catch-all for very large queues.
var (
	matchSize = [...]int{128, 512, 2048, 16384, 0}
	matchPool [len(matchSize)]sync.Pool
)

// get returns a machine to use for matching re.
// It uses the re's machine cache if possible, to avoid
// unnecessary allocation.
func (re *Regexp) get() *machine {
	m, ok := matchPool[re.mpool].Get().(*machine)
	if !ok {
		m = new(machine)
	}
	m.re = re
	m.p = re.prog
	if cap(m.matchcap) < re.matchcap {
		m.matchcap = make([]int, re.matchcap)
		for _, t := range m.pool {
			t.cap = make([]int, re.matchcap)
		}
	}

	// Allocate queues if needed.
	// Or reallocate, for "large" match pool.
	n := matchSize[re.mpool]
	if n == 0 { // large pool
		n = len(re.prog.Inst)
	}
	if len(m.q0.sparse) < n {
		m.q0 = queue{make([]uint32, n), make([]entry, 0, n)}
		m.q1 = queue{make([]uint32, n), make([]entry, 0, n)}
	}
	return m
}

// put returns a machine to the correct machine pool.
func (re *Regexp) put(m *machine) {
	m.re = nil
	m.p = nil
	m.inputs.clear()
	matchPool[re.mpool].Put(m)
}

// minInputLen walks the regexp to find the minimum length of any matchable input.
func minInputLen(re *syntax.Regexp) int {
	switch re.Op {
	default:
		return 0
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL, syntax.OpCharClass:
		return 1
	case syntax.OpLiteral:
		l := 0
		for _, r := range re.Rune {
			if r == utf8.RuneError {
				l++
			} else {
				l += utf8.RuneLen(r)
			}
		}
		return l
	case syntax.OpCapture, syntax.OpPlus:
		return minInputLen(re.Sub[0])
	case syntax.OpRepeat:
		return re.Min * minInputLen(re.Sub[0])
	case syntax.OpConcat:
		l := 0
		for _, sub := range re.Sub {
			l += minInputLen(sub)
		}
		return l
	case syntax.OpAlternate:
		l := minInputLen(re.Sub[0])
		var lnext int
		for _, sub := range re.Sub[1:] {
			lnext = minInputLen(sub)
			if lnext < l {
				l = lnext
			}
		}
		return l
	}
}

// MustCompile is like [Compile] but panics if the expression cannot be parsed.
// It simplifies safe initialization of global variables holding compiled regular
// expressions.
func MustCompile(str string) *Regexp {
	regexp, err := Compile(str)
	if err != nil {
		panic(`regexp: Compile(` + quote(str) + `): ` + err.Error())
	}
	return regexp
}

// MustCompilePOSIX is like [CompilePOSIX] but panics if the expression cannot be parsed.
// It simplifies safe initialization of global variables holding compiled regular
// expressions.
func MustCompilePOSIX(str string) *Regexp {
	regexp, err := CompilePOSIX(str)
	if err != nil {
		panic(`regexp: CompilePOSIX(` + quote(str) + `): ` + err.Error())
	}
	return regexp
}

func quote(s string) string {
	if strconv.CanBackquote(s) {
		return "`" + s + "`"
	}
	return strconv.Quote(s)
}

// NumSubexp returns the number of parenthesized subexpressions in this [Regexp].
func (re *Regexp) NumSubexp() int {
	return re.numSubexp
}

// SubexpNames returns the names of the parenthesized subexpressions
// in this [Regexp]. The name for the first sub-expression is names[1],
// so that if m is a match slice, the name for m[i] is SubexpNames()[i].
// Since the Regexp as a whole cannot be named, names[0] is always
// the empty string. The slice should not be modified.
func (re *Regexp) SubexpNames() []string {
	return re.subexpNames
}

// SubexpIndex returns the index of the first subexpression with the given name,
// or -1 if there is no subexpression with that name.
//
// Note that multiple subexpressions can be written using the same name, as in
// (?P<bob>a+)(?P<bob>b+), which declares two subexpressions named "bob".
// In this case, SubexpIndex returns the index of the leftmost such subexpression
// in the regular expression.
func (re *Regexp) SubexpIndex(name string) int {
	if name != "" {
		for i, s := range re.subexpNames {
			if name == s {
				return i
			}
		}
	}
	return -1
}

const endOfText rune = -1

// input abstracts different representations of the input text. It provides
// one-character lookahead.
type input interface {
	step(pos int) (r rune, width int) // advance one rune
	canCheckPrefix() bool             // can we look ahead without losing info?
	hasPrefix(re *Regexp) bool
	index(re *Regexp, pos int) int
	context(pos int) lazyFlag
}

// inputString scans a string.
type inputString struct {
	str string
}

func (i *inputString) step(pos int) (rune, int) {
	if pos < len(i.str) {
		c := i.str[pos]
		if c < utf8.RuneSelf {
			return rune(c), 1
		}
		return utf8.DecodeRuneInString(i.str[pos:])
	}
	return endOfText, 0
}

func (i *inputString) canCheckPrefix() bool {
	return true
}

func (i *inputString) hasPrefix(re *Regexp) bool {
	return strings.HasPrefix(i.str, re.prefix)
}

func (i *inputString) index(re *Regexp, pos int) int {
	return strings.Index(i.str[pos:], re.prefix)
}

func (i *inputString) context(pos int) lazyFlag {
	r1, r2 := endOfText, endOfText
	// 0 < pos && pos <= len(i.str)
	if uint(pos-1) < uint(len(i.str)) {
		r1 = rune(i.str[pos-1])
		if r1 >= utf8.RuneSelf {
			r1, _ = utf8.DecodeLastRuneInString(i.str[:pos])
		}
	}
	// 0 <= pos && pos < len(i.str)
	if uint(pos) < uint(len(i.str)) {
		r2 = rune(i.str[pos])
		if r2 >= utf8.RuneSelf {
			r2, _ = utf8.DecodeRuneInString(i.str[pos:])
		}
	}
	return newLazyFlag(r1, r2)
}

// inputBytes scans a byte slice.
type inputBytes struct {
	str *bytes.Buffer
}

func (i *inputBytes) step(pos int) (rune, int) {
	if pos < i.str.Len() {
		c := i.str.Bytes()[pos] // i.str[pos]
		if c < utf8.RuneSelf {
			return rune(c), 1
		}
		return utf8.DecodeRune(i.str.Bytes()[pos:])
	}
	return endOfText, 0
}

func (i *inputBytes) canCheckPrefix() bool {
	return true
}

func (i *inputBytes) hasPrefix(re *Regexp) bool {
	return bytes.HasPrefix(i.str.Bytes(), re.prefixBytes)
}

func (i *inputBytes) index(re *Regexp, pos int) int {
	return bytes.Index(i.str.Bytes()[pos:], re.prefixBytes)
}

func (i *inputBytes) context(pos int) lazyFlag {
	r1, r2 := endOfText, endOfText
	// 0 < pos && pos <= len(i.str)
	if uint(pos-1) < uint(i.str.Len()) {
		r1 = rune(i.str.Bytes()[pos-1])
		if r1 >= utf8.RuneSelf {
			r1, _ = utf8.DecodeLastRune(i.str.Bytes()[:pos])
		}
	}
	// 0 <= pos && pos < len(i.str)
	if uint(pos) < uint(i.str.Len()) {
		r2 = rune(i.str.Bytes()[pos])
		if r2 >= utf8.RuneSelf {
			r2, _ = utf8.DecodeRune(i.str.Bytes()[pos:])
		}
	}
	return newLazyFlag(r1, r2)
}

// inputReader scans a RuneReader.
type inputReader struct {
	r     io.RuneReader
	atEOT bool
	pos   int
}

func (i *inputReader) step(pos int) (rune, int) {
	if !i.atEOT && pos != i.pos {
		return endOfText, 0

	}
	r, w, err := i.r.ReadRune()
	if err != nil {
		i.atEOT = true
		return endOfText, 0
	}
	i.pos += w
	return r, w
}

func (i *inputReader) canCheckPrefix() bool {
	return false
}

func (i *inputReader) hasPrefix(re *Regexp) bool {
	return false
}

func (i *inputReader) index(re *Regexp, pos int) int {
	return -1
}

func (i *inputReader) context(pos int) lazyFlag {
	return 0 // not used
}

// LiteralPrefix returns a literal string that must begin any match
// of the regular expression re. It returns the boolean true if the
// literal string comprises the entire regular expression.
func (re *Regexp) LiteralPrefix() (prefix string, complete bool) {
	return re.prefix, re.prefixComplete
}

// Bitmap used by func special to check whether a character needs to be escaped.
var specialBytes [16]byte

// special reports whether byte b needs to be escaped by QuoteMeta.
func special(b byte) bool {
	return b < utf8.RuneSelf && specialBytes[b%16]&(1<<(b/16)) != 0
}

func init() {
	for _, b := range []byte(`\.+*?()|[]{}^$`) {
		specialBytes[b%16] |= 1 << (b / 16)
	}
}

// QuoteMeta returns a string that escapes all regular expression metacharacters
// inside the argument text; the returned string is a regular expression matching
// the literal text.
func QuoteMeta(s string) string {
	// A byte loop is correct because all metacharacters are ASCII.
	var i int
	for i = 0; i < len(s); i++ {
		if special(s[i]) {
			break
		}
	}
	// No meta characters found, so return original string.
	if i >= len(s) {
		return s
	}

	b := make([]byte, 2*len(s)-i)
	copy(b, s[:i])
	j := i
	for ; i < len(s); i++ {
		if special(s[i]) {
			b[j] = '\\'
			j++
		}
		b[j] = s[i]
		j++
	}
	return string(b[:j])
}

// The number of capture values in the program may correspond
// to fewer capturing expressions than are in the regexp.
// For example, "(a){0}" turns into an empty program, so the
// maximum capture in the program is 0 but we need to return
// an expression for \1.  Pad appends -1s to the slice a as needed.
func (re *Regexp) pad(a []int) []int {
	if a == nil {
		// No match.
		return nil
	}
	n := (1 + re.numSubexp) * 2
	for len(a) < n {
		a = append(a, -1)
	}
	return a
}

// allMatches calls deliver at most n times
// with the location of successive matches in the input text.
// The input text is b if non-nil, otherwise s.
func (re *Regexp) allMatches(s string, b []byte, n int, deliver func([]int)) {
	var end int
	if b == nil {
		end = len(s)
	} else {
		end = len(b)
	}

	for pos, i, prevMatchEnd := 0, 0, -1; i < n && pos <= end; {
		matches := re.doExecute(nil, b, s, pos, re.prog.NumCap, nil)
		if len(matches) == 0 {
			break
		}

		accept := true
		if matches[1] == pos {
			// We've found an empty match.
			if matches[0] == prevMatchEnd {
				// We don't allow an empty match right
				// after a previous match, so ignore it.
				accept = false
			}
			var width int
			if b == nil {
				is := inputString{str: s}
				_, width = is.step(pos)
			} else {
				ib := inputBytes{str: bytes.NewBuffer(b)}
				_, width = ib.step(pos)
			}
			if width > 0 {
				pos += width
			} else {
				pos = end + 1
			}
		} else {
			pos = matches[1]
		}
		prevMatchEnd = matches[1]

		if accept {
			deliver(re.pad(matches))
			i++
		}
	}
}

// Find returns a slice holding the text of the leftmost match in b of the regular expression.
// A return value of nil indicates no match.
func (re *Regexp) Find(b []byte) []byte {
	var dstCap [2]int
	a := re.doExecute(nil, b, "", 0, 2, dstCap[:0])
	if a == nil {
		return nil
	}
	return b[a[0]:a[1]:a[1]]
}

// FindIndex returns a two-element slice of integers defining the location of
// the leftmost match in b of the regular expression. The match itself is at
// b[loc[0]:loc[1]].
// A return value of nil indicates no match.
func (re *Regexp) FindIndex(b []byte) (loc []int) {
	a := re.doExecute(nil, b, "", 0, 2, nil)
	if a == nil {
		return nil
	}
	return a[0:2]
}

// FindString returns a string holding the text of the leftmost match in s of the regular
// expression. If there is no match, the return value is an empty string,
// but it will also be empty if the regular expression successfully matches
// an empty string. Use [Regexp.FindStringIndex] or [Regexp.FindStringSubmatch] if it is
// necessary to distinguish these cases.
func (re *Regexp) FindString(s string) string {
	var dstCap [2]int
	a := re.doExecute(nil, nil, s, 0, 2, dstCap[:0])
	if a == nil {
		return ""
	}
	return s[a[0]:a[1]]
}

// FindStringIndex returns a two-element slice of integers defining the
// location of the leftmost match in s of the regular expression. The match
// itself is at s[loc[0]:loc[1]].
// A return value of nil indicates no match.
func (re *Regexp) FindStringIndex(s string) (loc []int) {
	a := re.doExecute(nil, nil, s, 0, 2, nil)
	if a == nil {
		return nil
	}
	return a[0:2]
}

// FindReaderIndex returns a two-element slice of integers defining the
// location of the leftmost match of the regular expression in text read from
// the [io.RuneReader]. The match text was found in the input stream at
// byte offset loc[0] through loc[1]-1.
// A return value of nil indicates no match.
func (re *Regexp) FindReaderIndex(r io.RuneReader) (loc []int) {
	a := re.doExecute(r, nil, "", 0, 2, nil)
	if a == nil {
		return nil
	}
	return a[0:2]
}

// FindSubmatch returns a slice of slices holding the text of the leftmost
// match of the regular expression in b and the matches, if any, of its
// subexpressions, as defined by the 'Submatch' descriptions in the package
// comment.
// A return value of nil indicates no match.
func (re *Regexp) FindSubmatch(b []byte) [][]byte {
	var dstCap [4]int
	a := re.doExecute(nil, b, "", 0, re.prog.NumCap, dstCap[:0])
	if a == nil {
		return nil
	}
	ret := make([][]byte, 1+re.numSubexp)
	for i := range ret {
		if 2*i < len(a) && a[2*i] >= 0 {
			ret[i] = b[a[2*i]:a[2*i+1]:a[2*i+1]]
		}
	}
	return ret
}

// FindSubmatchIndex returns a slice holding the index pairs identifying the
// leftmost match of the regular expression in b and the matches, if any, of
// its subexpressions, as defined by the 'Submatch' and 'Index' descriptions
// in the package comment.
// A return value of nil indicates no match.
func (re *Regexp) FindSubmatchIndex(b []byte) []int {
	return re.pad(re.doExecute(nil, b, "", 0, re.prog.NumCap, nil))
}

// FindStringSubmatch returns a slice of strings holding the text of the
// leftmost match of the regular expression in s and the matches, if any, of
// its subexpressions, as defined by the 'Submatch' description in the
// package comment.
// A return value of nil indicates no match.
func (re *Regexp) FindStringSubmatch(s string) []string {
	var dstCap [4]int
	a := re.doExecute(nil, nil, s, 0, re.prog.NumCap, dstCap[:0])
	if a == nil {
		return nil
	}
	ret := make([]string, 1+re.numSubexp)
	for i := range ret {
		if 2*i < len(a) && a[2*i] >= 0 {
			ret[i] = s[a[2*i]:a[2*i+1]]
		}
	}
	return ret
}

// FindStringSubmatchIndex returns a slice holding the index pairs
// identifying the leftmost match of the regular expression in s and the
// matches, if any, of its subexpressions, as defined by the 'Submatch' and
// 'Index' descriptions in the package comment.
// A return value of nil indicates no match.
func (re *Regexp) FindStringSubmatchIndex(s string) []int {
	return re.pad(re.doExecute(nil, nil, s, 0, re.prog.NumCap, nil))
}

// FindReaderSubmatchIndex returns a slice holding the index pairs
// identifying the leftmost match of the regular expression of text read by
// the [io.RuneReader], and the matches, if any, of its subexpressions, as defined
// by the 'Submatch' and 'Index' descriptions in the package comment. A
// return value of nil indicates no match.
func (re *Regexp) FindReaderSubmatchIndex(r io.RuneReader) []int {
	return re.pad(re.doExecute(r, nil, "", 0, re.prog.NumCap, nil))
}

const startSize = 10 // The size at which to start a slice in the 'All' routines.

// FindAll is the 'All' version of [Regexp.Find]; it returns a slice of all successive
// matches of the expression, as defined by the 'All' description in the
// package comment.
// A return value of nil indicates no match.
func (re *Regexp) FindAll(b []byte, n int) [][]byte {
	if n < 0 {
		n = len(b) + 1
	}
	var result [][]byte
	re.allMatches("", b, n, func(match []int) {
		if result == nil {
			result = make([][]byte, 0, startSize)
		}
		result = append(result, b[match[0]:match[1]:match[1]])
	})
	return result
}

// FindAllIndex is the 'All' version of [Regexp.FindIndex]; it returns a slice of all
// successive matches of the expression, as defined by the 'All' description
// in the package comment.
// A return value of nil indicates no match.
func (re *Regexp) FindAllIndex(b []byte, n int) [][]int {
	if n < 0 {
		n = len(b) + 1
	}
	var result [][]int
	re.allMatches("", b, n, func(match []int) {
		if result == nil {
			result = make([][]int, 0, startSize)
		}
		result = append(result, match[0:2])
	})
	return result
}

// FindAllString is the 'All' version of [Regexp.FindString]; it returns a slice of all
// successive matches of the expression, as defined by the 'All' description
// in the package comment.
// A return value of nil indicates no match.
func (re *Regexp) FindAllString(s string, n int) []string {
	if n < 0 {
		n = len(s) + 1
	}
	var result []string
	re.allMatches(s, nil, n, func(match []int) {
		if result == nil {
			result = make([]string, 0, startSize)
		}
		result = append(result, s[match[0]:match[1]])
	})
	return result
}

// FindAllStringIndex is the 'All' version of [Regexp.FindStringIndex]; it returns a
// slice of all successive matches of the expression, as defined by the 'All'
// description in the package comment.
// A return value of nil indicates no match.
func (re *Regexp) FindAllStringIndex(s string, n int) [][]int {
	if n < 0 {
		n = len(s) + 1
	}
	var result [][]int
	re.allMatches(s, nil, n, func(match []int) {
		if result == nil {
			result = make([][]int, 0, startSize)
		}
		result = append(result, match[0:2])
	})
	return result
}

// FindAllSubmatch is the 'All' version of [Regexp.FindSubmatch]; it returns a slice
// of all successive matches of the expression, as defined by the 'All'
// description in the package comment.
// A return value of nil indicates no match.
func (re *Regexp) FindAllSubmatch(b []byte, n int) [][][]byte {
	if n < 0 {
		n = len(b) + 1
	}
	var result [][][]byte
	re.allMatches("", b, n, func(match []int) {
		if result == nil {
			result = make([][][]byte, 0, startSize)
		}
		slice := make([][]byte, len(match)/2)
		for j := range slice {
			if match[2*j] >= 0 {
				slice[j] = b[match[2*j]:match[2*j+1]:match[2*j+1]]
			}
		}
		result = append(result, slice)
	})
	return result
}

// FindAllSubmatchIndex is the 'All' version of [Regexp.FindSubmatchIndex]; it returns
// a slice of all successive matches of the expression, as defined by the
// 'All' description in the package comment.
// A return value of nil indicates no match.
func (re *Regexp) FindAllSubmatchIndex(b []byte, n int) [][]int {
	if n < 0 {
		n = len(b) + 1
	}
	var result [][]int
	re.allMatches("", b, n, func(match []int) {
		if result == nil {
			result = make([][]int, 0, startSize)
		}
		result = append(result, match)
	})
	return result
}

// FindAllStringSubmatch is the 'All' version of [Regexp.FindStringSubmatch]; it
// returns a slice of all successive matches of the expression, as defined by
// the 'All' description in the package comment.
// A return value of nil indicates no match.
func (re *Regexp) FindAllStringSubmatch(s string, n int) [][]string {
	if n < 0 {
		n = len(s) + 1
	}
	var result [][]string
	re.allMatches(s, nil, n, func(match []int) {
		if result == nil {
			result = make([][]string, 0, startSize)
		}
		slice := make([]string, len(match)/2)
		for j := range slice {
			if match[2*j] >= 0 {
				slice[j] = s[match[2*j]:match[2*j+1]]
			}
		}
		result = append(result, slice)
	})
	return result
}

// FindAllStringSubmatchIndex is the 'All' version of
// [Regexp.FindStringSubmatchIndex]; it returns a slice of all successive matches of
// the expression, as defined by the 'All' description in the package
// comment.
// A return value of nil indicates no match.
func (re *Regexp) FindAllStringSubmatchIndex(s string, n int) [][]int {
	if n < 0 {
		n = len(s) + 1
	}
	var result [][]int
	re.allMatches(s, nil, n, func(match []int) {
		if result == nil {
			result = make([][]int, 0, startSize)
		}
		result = append(result, match)
	})
	return result
}

// AppendText implements [encoding.TextAppender]. The output
// matches that of calling the [Regexp.String] method.
//
// Note that the output is lossy in some cases: This method does not indicate
// POSIX regular expressions (i.e. those compiled by calling [CompilePOSIX]), or
// those for which the [Regexp.Longest] method has been called.
func (re *Regexp) AppendText(b []byte) ([]byte, error) {
	return append(b, re.String()...), nil
}

// MarshalText implements [encoding.TextMarshaler]. The output
// matches that of calling the [Regexp.AppendText] method.
//
// See [Regexp.AppendText] for more information.
func (re *Regexp) MarshalText() ([]byte, error) {
	return re.AppendText(nil)
}

// UnmarshalText implements [encoding.TextUnmarshaler] by calling
// [Compile] on the encoded value.
func (re *Regexp) UnmarshalText(text []byte) error {
	newRE, err := Compile(string(text))
	if err != nil {
		return err
	}
	*re = *newRE
	return nil
}
