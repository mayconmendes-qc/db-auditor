package migrationrisk

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Token kinds for the DDL subset lexer.
const (
	tokEOF = iota
	tokIdent
	tokNumber
	tokString
	tokDollar
	tokSymbol
	tokComment
)

type token struct {
	kind  int
	val   string
	line  int
	start int
}

type lexer struct {
	src  string
	pos  int
	line int
}

func newLexer(src string) *lexer {
	return &lexer{src: src, line: 1}
}

func (l *lexer) next() token {
	l.skipSpace()
	if l.pos >= len(l.src) {
		return token{kind: tokEOF, line: l.line, start: l.pos}
	}
	start, line := l.pos, l.line
	r, size := utf8.DecodeRuneInString(l.src[l.pos:])
	switch {
	case r == '-' && l.peek(1) == '-':
		return l.lineComment(start, line)
	case r == '/' && l.peek(1) == '*':
		return l.blockComment(start, line)
	case r == '\'':
		return l.quotedString(start, line)
	case r == 'E' || r == 'e':
		if l.peek(1) == '\'' {
			l.pos++
			return l.quotedString(start, line)
		}
		return l.ident(start, line)
	case r == '$':
		return l.dollarOrSymbol(start, line)
	case unicode.IsLetter(r) || r == '_':
		return l.ident(start, line)
	case unicode.IsDigit(r):
		return l.number(start, line)
	default:
		l.pos += size
		if r == '\n' {
			l.line++
		}
		return token{kind: tokSymbol, val: string(r), line: line, start: start}
	}
}

func (l *lexer) peek(offset int) byte {
	i := l.pos + offset
	if i < 0 || i >= len(l.src) {
		return 0
	}
	return l.src[i]
}

func (l *lexer) skipSpace() {
	for l.pos < len(l.src) {
		r, size := utf8.DecodeRuneInString(l.src[l.pos:])
		if r == '\n' {
			l.line++
			l.pos += size
			continue
		}
		if unicode.IsSpace(r) {
			l.pos += size
			continue
		}
		return
	}
}

func (l *lexer) lineComment(start, line int) token {
	l.pos += 2
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.pos++
	}
	return token{kind: tokComment, val: l.src[start:l.pos], line: line, start: start}
}

func (l *lexer) blockComment(start, line int) token {
	l.pos += 2
	for l.pos+1 < len(l.src) {
		if l.src[l.pos] == '*' && l.src[l.pos+1] == '/' {
			l.pos += 2
			return token{kind: tokComment, val: l.src[start:l.pos], line: line, start: start}
		}
		if l.src[l.pos] == '\n' {
			l.line++
		}
		l.pos++
	}
	l.pos = len(l.src)
	return token{kind: tokComment, val: l.src[start:l.pos], line: line, start: start}
}

func (l *lexer) quotedString(start, line int) token {
	// Caller positioned at opening quote (or after E for E'').
	if l.pos < len(l.src) && l.src[l.pos] == '\'' {
		l.pos++
	}
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == '\'' {
			l.pos++
			if l.pos < len(l.src) && l.src[l.pos] == '\'' {
				l.pos++
				continue
			}
			return token{kind: tokString, val: "?", line: line, start: start}
		}
		if ch == '\n' {
			l.line++
		}
		l.pos++
	}
	return token{kind: tokString, val: "?", line: line, start: start}
}

func (l *lexer) dollarOrSymbol(start, line int) token {
	// $tag$ ... $tag$ or bare $
	i := l.pos + 1
	for i < len(l.src) {
		ch := l.src[i]
		if ch == '$' {
			tag := l.src[l.pos : i+1]
			rest := l.src[i+1:]
			end := strings.Index(rest, tag)
			if end < 0 {
				l.pos = len(l.src)
				return token{kind: tokDollar, val: "?", line: line, start: start}
			}
			body := rest[:end]
			l.pos = i + 1 + end + len(tag)
			l.line += strings.Count(body, "\n")
			return token{kind: tokDollar, val: "?", line: line, start: start}
		}
		if !(unicode.IsLetter(rune(ch)) || unicode.IsDigit(rune(ch)) || ch == '_') {
			break
		}
		i++
	}
	l.pos++
	return token{kind: tokSymbol, val: "$", line: line, start: start}
}

func (l *lexer) ident(start, line int) token {
	for l.pos < len(l.src) {
		r, size := utf8.DecodeRuneInString(l.src[l.pos:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$' {
			l.pos += size
			continue
		}
		// Keep schema.table as one identifier for DDL classification.
		if r == '.' && l.pos+size < len(l.src) {
			next, _ := utf8.DecodeRuneInString(l.src[l.pos+size:])
			if unicode.IsLetter(next) || next == '_' {
				l.pos += size
				continue
			}
		}
		break
	}
	return token{kind: tokIdent, val: strings.ToLower(l.src[start:l.pos]), line: line, start: start}
}

func (l *lexer) number(start, line int) token {
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if (ch >= '0' && ch <= '9') || ch == '.' {
			l.pos++
			continue
		}
		break
	}
	return token{kind: tokNumber, val: "?", line: line, start: start}
}

// SplitStatements returns statement texts with starting line numbers,
// preserving comments that precede each statement for suppress directives.
func SplitStatements(src string) []struct {
	Text string
	Line int
} {
	var out []struct {
		Text string
		Line int
	}
	l := newLexer(src)
	var b strings.Builder
	startLine := 1
	empty := true
	for {
		tok := l.next()
		if tok.kind == tokEOF {
			if !empty {
				out = append(out, struct {
					Text string
					Line int
				}{Text: strings.TrimSpace(b.String()), Line: startLine})
			}
			return out
		}
		if empty {
			startLine = tok.line
		}
		if tok.kind == tokSymbol && tok.val == ";" {
			text := strings.TrimSpace(b.String())
			if text != "" {
				out = append(out, struct {
					Text string
					Line int
				}{Text: text, Line: startLine})
			}
			b.Reset()
			empty = true
			continue
		}
		if tok.kind == tokComment {
			if !empty {
				b.WriteByte(' ')
			}
			b.WriteString(tok.val)
			// Preserve a newline after -- comments so literal stripping cannot
			// swallow the following DDL when tokens were joined with spaces.
			if strings.HasPrefix(strings.TrimSpace(tok.val), "--") {
				b.WriteByte('\n')
			}
			empty = false
			continue
		}
		if tok.kind == tokString || tok.kind == tokDollar || tok.kind == tokNumber {
			if !empty {
				b.WriteByte(' ')
			}
			b.WriteString("?")
			empty = false
			continue
		}
		if !empty {
			b.WriteByte(' ')
		}
		b.WriteString(tok.val)
		empty = false
	}
}
