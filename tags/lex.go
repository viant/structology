package tags

import (
	"github.com/viant/parsly"
	"github.com/viant/parsly/matcher"
)

const (
	whitespaceToken = iota
	comaTerminatorToken
	scopeBlockToken

	quotedToken
)

var (
	whitespaceMatcher     = parsly.NewToken(whitespaceToken, " ", matcher.NewWhiteSpace())
	comaTerminatorMatcher = parsly.NewToken(comaTerminatorToken, "coma", matcher.NewTerminator(',', true))
	scopeBlockMatcher     = parsly.NewToken(scopeBlockToken, "{ .... }", matcher.NewBlock('{', '}', '\\'))

	quotedMatcher = parsly.NewToken(quotedToken, "' .... '", matcher.NewQuote('\'', '\\'))
)
