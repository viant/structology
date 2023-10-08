package format

import (
	"github.com/viant/parsly"
	"github.com/viant/parsly/matcher"
)

const (
	comaTerminatorToken = iota
	scopeBlockToken
)

var (
	comaTerminatorMatcher = parsly.NewToken(comaTerminatorToken, "coma", matcher.NewTerminator(',', true))
	scopeBlockMatcher     = parsly.NewToken(scopeBlockToken, "{ .... }", matcher.NewBlock('{', '}', '\\'))
)
