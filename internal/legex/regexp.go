package legex

import (
	"regexp/syntax"
	"strconv"
	"unicode/utf8"
)

// Regexp is the representation of a compiled regular expression.
// A Regexp is safe for concurrent use by multiple goroutines,
// except for configuration methods, such as [Regexp.Longest].
type Regexp struct {
	expr        string       // as passed to Compile
	prog        *syntax.Prog // compiled program
	numSubexp   int
	subexpNames []string
	prefix      string // required prefix in unanchored matches
	// prefixEnd      uint32         // pc for last rune in prefix
	mpool          int            // pool for machines
	matchcap       int            // size of recorded match lengths
	prefixComplete bool           // prefix is the entire regexp
	cond           syntax.EmptyOp // empty-width conditions required at start of match

	// This field can be modified by the Longest method,
	// but it is otherwise read-only.
	longest bool // whether regexp prefers leftmost-longest match
}

// String returns the source text used to compile the regular expression.
func (re *Regexp) String() string {
	return re.expr
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

	matchcap := max(prog.NumCap, 2)
	regexp := &Regexp{
		expr:        expr,
		prog:        prog,
		numSubexp:   maxCap,
		subexpNames: capNames,
		cond:        prog.StartCond(),
		longest:     longest,
		matchcap:    matchcap,
	}
	regexp.prefix, regexp.prefixComplete = prog.Prefix()

	n := len(prog.Inst)
	i := 0
	for defaultSize[i] != 0 && defaultSize[i] < n {
		i++
	}
	regexp.mpool = i

	return regexp, nil
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

// // The number of capture values in the program may correspond
// // to fewer capturing expressions than are in the regexp.
// // For example, "(a){0}" turns into an empty program, so the
// // maximum capture in the program is 0 but we need to return
// // an expression for \1.  Pad appends -1s to the slice a as needed.
// func (re *Regexp) pad(a []int) []int {
// 	if a == nil {
// 		// No match.
// 		return nil
// 	}
// 	n := (1 + re.numSubexp) * 2
// 	for len(a) < n {
// 		a = append(a, -1)
// 	}
// 	return a
// }

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
