package luasrc

import (
	"fmt"
	"math"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// Call is a call statement of a bare global function: `Name(args)`, with the tokens of each argument.
type Call struct {
	Name  string
	Args  [][]Token
	Start int // byte offsets of the statement, with a `;` that directly follows it
	End   int
}

// Function is a top-level `function Name(...) ... end` and the call statements directly in its body.
type Function struct {
	Name     string
	Start    int // byte offsets of the whole declaration
	End      int
	EndStart int // where its closing `end` begins
	Calls    []Call
}

func luaError(message, file string) error {
	return &diag.Error{
		Msg:  "Cannot safely read map Lua: " + message,
		File: file,
		Hint: "Re-save the map in World Editor to restore its generated Lua structure.",
	}
}

// Only prefix expressions can be assignment targets or call statements. A direct call loses its editable identity
// as soon as another suffix or operator follows.
type expression struct {
	bare       string
	hasBare    bool
	assignable bool
	call       bool
	direct     *Call
}

var precedence = map[string]int{
	"or": 1, "and": 2,
	"<": 3, ">": 3, "<=": 3, ">=": 3, "~=": 3, "==": 3,
	"|": 4, "~": 5, "&": 6, "<<": 7, ">>": 7, "..": 8,
	"+": 9, "-": 9, "*": 10, "/": 10, "//": 10, "%": 10, "^": 12,
}

// structureError carries a failure out of the recursive reader.
type structureError struct{ err error }

type reader struct {
	source    string
	tokens    []Token
	file      string
	at        int
	depth     int
	functions []Function
}

func (r *reader) value() string {
	if r.at < len(r.tokens) {
		return r.tokens[r.at].Raw
	}
	return ""
}

func (r *reader) kind() (Kind, bool) {
	if r.at < len(r.tokens) {
		return r.tokens[r.at].Kind, true
	}
	return 0, false
}

func (r *reader) take(value string) bool {
	if r.value() != value {
		return false
	}
	r.at++
	return true
}

func (r *reader) expect(value string) Token {
	if r.value() != value {
		r.fail("expected '" + value + "'")
	}
	token := r.tokens[r.at]
	r.at++
	return token
}

func (r *reader) name() string {
	if kind, ok := r.kind(); !ok || kind != Name || keywords[r.value()] {
		r.fail("expected a name")
	}
	r.at++
	return r.tokens[r.at-1].Raw
}

func (r *reader) fail(message string) {
	offset := len(r.source)
	if r.at < len(r.tokens) {
		offset = r.tokens[r.at].Start
	}
	// Positions are counted as JavaScript counts them, in UTF-16 units.
	message = fmt.Sprintf("%s at character %d", message, text.UTF16Offset(r.source, offset))
	panic(structureError{luaError(message, r.file)})
}

func (r *reader) enter() {
	// Bound both block and expression recursion.
	r.depth++
	if r.depth > 200 {
		r.fail("nesting is too deep to establish safe edit boundaries")
	}
}

func (r *reader) chunk(stops []string, calls *[]Call, root bool) {
	r.enter()
	for r.at < len(r.tokens) && !slices.Contains(stops, r.value()) {
		if r.take("return") {
			if r.value() != "" && !slices.Contains(stops, r.value()) && r.value() != ";" {
				r.expressions()
			}
			r.take(";")
			if r.at < len(r.tokens) && !slices.Contains(stops, r.value()) {
				r.fail("return must end its block")
			}
			break
		}
		r.statement(calls, root)
	}
	if !root && r.at == len(r.tokens) {
		r.fail("unterminated block")
	}
	r.depth--
}

func (r *reader) block() {
	r.chunk([]string{"end"}, nil, false)
	r.expect("end")
}

func (r *reader) functionBody(calls *[]Call) Token {
	r.expect("(")
	if !r.take(")") {
		for {
			if r.take("...") {
				break
			}
			r.name()
			if !r.take(",") {
				break
			}
		}
		r.expect(")")
	}
	r.chunk([]string{"end"}, calls, false)
	return r.expect("end")
}

func (r *reader) statement(calls *[]Call, root bool) {
	start := r.tokens[r.at].Start
	switch {
	case r.take(";"):
	case r.take("function"):
		name := r.name()
		bare := true
		for r.take(".") {
			bare = false
			r.name()
		}
		if r.take(":") {
			bare = false
			r.name()
		}
		if root && bare {
			direct := []Call{}
			end := r.functionBody(&direct)
			r.functions = append(r.functions, Function{
				Name: name, Start: start, End: end.End, EndStart: end.Start, Calls: direct,
			})
		} else {
			r.functionBody(nil)
		}
	case r.take("local"):
		if r.take("function") {
			r.name()
			r.functionBody(nil)
		} else {
			r.name()
			for r.take(",") {
				r.name()
			}
			if r.take("=") {
				r.expressions()
			}
		}
	case r.take("if"):
		r.expression(1)
		r.expect("then")
		branches := []string{"elseif", "else", "end"}
		r.chunk(branches, nil, false)
		for r.take("elseif") {
			r.expression(1)
			r.expect("then")
			r.chunk(branches, nil, false)
		}
		if r.take("else") {
			r.chunk([]string{"end"}, nil, false)
		}
		r.expect("end")
	case r.take("while"):
		r.expression(1)
		r.expect("do")
		r.block()
	case r.take("for"):
		r.name()
		if r.take("=") {
			r.expression(1)
			r.expect(",")
			r.expression(1)
			if r.take(",") {
				r.expression(1)
			}
		} else {
			for r.take(",") {
				r.name()
			}
			r.expect("in")
			r.expressions()
		}
		r.expect("do")
		r.block()
	case r.take("do"):
		r.block()
	case r.take("repeat"):
		r.chunk([]string{"until"}, nil, false)
		r.expect("until")
		r.expression(1)
	case r.take("break"):
		// No expression follows break; semantic loop validation belongs to Lua.
	case r.take("goto"):
		r.name()
	case r.take("::"):
		r.name()
		r.expect("::")
	default:
		target := r.prefix()
		if r.value() == "=" || r.value() == "," {
			if !target.assignable {
				r.fail("invalid assignment target")
			}
			for r.take(",") {
				if !r.prefix().assignable {
					r.fail("invalid assignment target")
				}
			}
			r.expect("=")
			r.expressions()
			return
		}
		if !target.call {
			r.fail("expected an assignment or call statement")
		}
		if target.direct != nil && calls != nil {
			call := *target.direct
			// A skipped comment between a call and a semicolon must survive edits.
			if r.value() == ";" && text.Trim(r.source[call.End:r.tokens[r.at].Start]) == "" {
				call.End = r.tokens[r.at].End
				r.at++
			}
			*calls = append(*calls, call)
		}
	}
}

func (r *reader) expressions() {
	r.expression(1)
	for r.take(",") {
		r.expression(1)
	}
}

func (r *reader) expression(minimum int) {
	r.enter()
	kind, hasToken := r.kind()
	switch value := r.value(); {
	case value == "not" || value == "#" || value == "-" || value == "~":
		r.at++
		r.expression(11)
	case r.take("function"):
		r.functionBody(nil)
	case value == "{":
		r.table()
	case value == "nil" || value == "true" || value == "false" || value == "..." ||
		(hasToken && (kind == Number || kind == String)):
		r.at++
	default:
		r.prefix()
	}
	for {
		operator := precedence[r.value()]
		if operator < minimum || operator == 0 {
			break
		}
		r.at++
		// Consuming a right-associative chain iteratively keeps long `..` chains within the depth limit.
		r.expression(operator + 1)
	}
	r.depth--
}

func (r *reader) prefix() expression {
	start := 0
	if r.at < len(r.tokens) {
		start = r.tokens[r.at].Start
	}
	var current expression
	if r.take("(") {
		r.expression(1)
		r.expect(")")
	} else {
		current = expression{bare: r.name(), hasBare: true, assignable: true}
	}
	for {
		kind, hasToken := r.kind()
		switch {
		case r.take("["):
			r.expression(1)
			r.expect("]")
			current = expression{assignable: true}
		case r.take("."):
			r.name()
			current = expression{assignable: true}
		case r.take(":"):
			r.name()
			r.arguments()
			current = expression{call: true}
		case r.value() == "(" || r.value() == "{" || (hasToken && kind == String):
			args := r.arguments()
			next := expression{call: true}
			if current.hasBare {
				next.direct = &Call{Name: current.bare, Args: args, Start: start, End: r.tokens[r.at-1].End}
			}
			current = next
		default:
			return current
		}
	}
}

func (r *reader) arguments() [][]Token {
	args := [][]Token{}
	if r.take("(") {
		if !r.take(")") {
			for {
				start := r.at
				r.expression(1)
				args = append(args, r.tokens[start:r.at])
				if !r.take(",") {
					break
				}
			}
			r.expect(")")
		}
		return args
	}
	start := r.at
	if kind, ok := r.kind(); r.value() == "{" {
		r.table()
	} else if ok && kind == String {
		r.at++
	} else {
		r.fail("expected call arguments")
	}
	return append(args, r.tokens[start:r.at])
}

func (r *reader) table() {
	r.expect("{")
	for !r.take("}") {
		if r.take("[") {
			r.expression(1)
			r.expect("]")
			r.expect("=")
			r.expression(1)
		} else {
			if kind, ok := r.kind(); ok && kind == Name && r.at+1 < len(r.tokens) && r.tokens[r.at+1].Raw == "=" {
				r.name()
				r.expect("=")
			}
			r.expression(1)
		}
		if !r.take(",") && !r.take(";") {
			r.expect("}")
			break
		}
	}
}

// Functions reads the top-level function declarations of Lua source and the call statements directly in each,
// without evaluating Lua. Anything it cannot read with certainty is an error that names file.
func Functions(source, file string) (functions []Function, err error) {
	tokens, fault := Tokenize(source)
	if fault != nil {
		return nil, luaError(fault.Msg, file)
	}
	r := &reader{source: source, tokens: tokens, file: file}
	defer func() {
		if recovered := recover(); recovered != nil {
			failure, ok := recovered.(structureError)
			if !ok {
				panic(recovered)
			}
			functions, err = nil, failure.err
		}
	}()
	r.chunk(nil, nil, true)
	return r.functions, nil
}

var literalNumber = regexp.MustCompile(`^(?:0[xX][0-9a-fA-F]+|(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?)$`)

// LiteralNumber returns the value of tokens when they are one literal decimal number or hexadecimal integer,
// optionally negated.
func LiteralNumber(tokens []Token) (float64, bool) {
	negative := len(tokens) == 2 && tokens[0].Kind == Symbol && tokens[0].Raw == "-"
	index, want := 0, 1
	if negative {
		index, want = 1, 2
	}
	if len(tokens) != want || tokens[index].Kind != Number || !literalNumber.MatchString(tokens[index].Raw) {
		return 0, false
	}
	raw := tokens[index].Raw
	var value float64
	if strings.HasPrefix(raw, "0x") || strings.HasPrefix(raw, "0X") {
		integer, _ := new(big.Int).SetString(raw[2:], 16)
		value, _ = new(big.Float).SetInt(integer).Float64()
	} else {
		value, _ = strconv.ParseFloat(raw, 64) // out of range gives an infinity, refused below
	}
	if negative {
		value = -value
	}
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return 0, false
	}
	return value, true
}

// PlayerID returns n when tokens are `Player(n)` with a literal whole number.
func PlayerID(tokens []Token) (int, bool) {
	last := len(tokens) - 1
	if len(tokens) < 3 || tokens[0].Kind != Name || tokens[0].Raw != "Player" ||
		tokens[1].Kind != Symbol || tokens[1].Raw != "(" ||
		tokens[last].Kind != Symbol || tokens[last].Raw != ")" {
		return 0, false
	}
	value, ok := LiteralNumber(tokens[2:last])
	if !ok || value != math.Trunc(value) {
		return 0, false
	}
	return int(value), true
}
