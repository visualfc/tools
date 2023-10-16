// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package completion

/*
import (
	"context"
	"go/types"
)

// addCandidate adds a completion candidate to suggestions, without searching
// its members for more candidates.
func (c *gopCompleter) addCandidate(ctx context.Context, cand *candidate) {
	obj := cand.obj
	if c.matchingCandidate(cand) {
		cand.score *= highScore

		if p := c.penalty(cand); p > 0 {
			cand.score *= (1 - p)
		}
	} else if isTypeName(obj) {
		// If obj is a *types.TypeName that didn't otherwise match, check
		// if a literal object of this type makes a good candidate.

		// We only care about named types (i.e. don't want builtin types).
		if _, isNamed := obj.Type().(*types.Named); isNamed {
			c.literal(ctx, obj.Type(), cand.imp)
		}
	}

	// Lower score of method calls so we prefer fields and vars over calls.
	if cand.hasMod(invoke) {
		if sig, ok := obj.Type().Underlying().(*types.Signature); ok && sig.Recv() != nil {
			cand.score *= 0.9
		}
	}

	// Prefer private objects over public ones.
	if !obj.Exported() && obj.Parent() != types.Universe {
		cand.score *= 1.1
	}

	// Slight penalty for index modifier (e.g. changing "foo" to
	// "foo[]") to curb false positives.
	if cand.hasMod(index) {
		cand.score *= 0.9
	}

	// Favor shallow matches by lowering score according to depth.
	cand.score -= cand.score * c.deepState.scorePenalty(cand)

	if cand.score < 0 {
		cand.score = 0
	}

	cand.name = deepCandName(cand)
	if item, err := c.item(ctx, *cand); err == nil {
		c.items = append(c.items, item)
	}
}

// penalty reports a score penalty for cand in the range (0, 1).
// For example, a candidate is penalized if it has already been used
// in another switch case statement.
func (c *gopCompleter) penalty(cand *candidate) float64 {
	for _, p := range c.inference.penalized {
		if c.objChainMatches(cand, p.objChain) {
			return p.penalty
		}
	}

	return 0
}

// objChainMatches reports whether cand combined with the surrounding
// object prefix matches chain.
func (c *gopCompleter) objChainMatches(cand *candidate, chain []types.Object) bool {
	// For example, when completing:
	//
	//   foo.ba<>
	//
	// If we are considering the deep candidate "bar.baz", cand is baz,
	// objChain is [foo] and deepChain is [bar]. We would match the
	// chain [foo, bar, baz].
	if len(chain) != len(c.inference.objChain)+len(cand.path)+1 {
		return false
	}

	if chain[len(chain)-1] != cand.obj {
		return false
	}

	for i, o := range c.inference.objChain {
		if chain[i] != o {
			return false
		}
	}

	for i, o := range cand.path {
		if chain[i+len(c.inference.objChain)] != o {
			return false
		}
	}

	return true
}
*/
