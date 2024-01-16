// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package completion

import (
	"context"
	"go/types"
	"strings"
	"time"

	"github.com/qiniu/x/log"
	"golang.org/x/tools/gopls/internal/goxls"
)

// deepSearch searches a candidate and its subordinate objects for completion
// items if deep completion is enabled and adds the valid candidates to
// completion items.
func (c *gopCompleter) deepSearch(ctx context.Context, start time.Time, deadline *time.Time) {
	defer func() {
		// We can return early before completing the search, so be sure to
		// clear out our queues to not impact any further invocations.
		c.deepState.thisQueue = c.deepState.thisQueue[:0]
		c.deepState.nextQueue = c.deepState.nextQueue[:0]
	}()
	if goxls.DbgCompletion {
		log.Println("gopCompleter.deepSearch: n =", len(c.deepState.nextQueue))
	}

	first := true // always fully process the first set of candidates
	for len(c.deepState.nextQueue) > 0 && (first || deadline == nil || time.Now().Before(*deadline)) {
		first = false
		c.deepState.thisQueue, c.deepState.nextQueue = c.deepState.nextQueue, c.deepState.thisQueue[:0]

	outer:
		for _, cand := range c.deepState.thisQueue {
			obj := cand.obj
			if obj == nil {
				continue
			}

			// At the top level, dedupe by object.
			if len(cand.path) == 0 {
				if c.seen[obj] {
					continue
				}
				c.seen[obj] = true
			}

			// If obj is not accessible because it lives in another package and is
			// not exported, don't treat it as a completion candidate unless it's
			// a package completion candidate.
			if !c.completionContext.packageCompletion &&
				obj.Pkg() != nil && obj.Pkg() != c.pkg.GetTypes() && !obj.Exported() {
				continue
			}

			// If we want a type name, don't offer non-type name candidates.
			// However, do offer package names since they can contain type names,
			// and do offer any candidate without a type since we aren't sure if it
			// is a type name or not (i.e. unimported candidate).
			if c.wantTypeName() && obj.Type() != nil && !isTypeName(obj) && !isPkgName(obj) {
				continue
			}

			// When searching deep, make sure we don't have a cycle in our chain.
			// We don't dedupe by object because we want to allow both "foo.Baz"
			// and "bar.Baz" even though "Baz" is represented the same types.Object
			// in both.
			for _, seenObj := range cand.path {
				if seenObj == obj {
					continue outer
				}
			}

			c.addCandidate(ctx, &cand)

			c.deepState.candidateCount++
			if c.opts.budget > 0 && c.deepState.candidateCount%100 == 0 {
				spent := float64(time.Since(start)) / float64(c.opts.budget)
				select {
				case <-ctx.Done():
					return
				default:
					// If we are almost out of budgeted time, no further elements
					// should be added to the queue. This ensures remaining time is
					// used for processing current queue.
					if !c.deepState.queueClosed && spent >= 0.85 {
						c.deepState.queueClosed = true
					}
				}
			}

			// if deep search is disabled, don't add any more candidates.
			if !c.deepState.enabled || c.deepState.queueClosed {
				continue
			}

			// Searching members for a type name doesn't make sense.
			if isTypeName(obj) {
				continue
			}
			if obj.Type() == nil {
				continue
			}

			// Don't search embedded fields because they were already included in their
			// parent's fields.
			if v, ok := obj.(*types.Var); ok && v.Embedded() {
				continue
			}

			if sig, ok := obj.Type().Underlying().(*types.Signature); ok {
				// If obj is a function that takes no arguments and returns one
				// value, keep searching across the function call.
				if sig.Params().Len() == 0 && sig.Results().Len() == 1 {
					path := c.deepState.newPath(cand, obj)
					// The result of a function call is not addressable.
					c.methodsAndFields(sig.Results().At(0).Type(), false, cand.imp, func(newCand candidate) {
						newCand.pathInvokeMask = cand.pathInvokeMask | (1 << uint64(len(cand.path)))
						newCand.path = path
						c.deepState.enqueue(newCand)
					})
				}
			}

			path := c.deepState.newPath(cand, obj)
			switch obj := obj.(type) {
			case *types.PkgName:
				c.packageMembers(obj.Imported(), stdScore, cand.imp, func(newCand candidate) {
					newCand.pathInvokeMask = cand.pathInvokeMask
					newCand.path = path
					c.deepState.enqueue(newCand)
				})
			default:
				// goxls: force cand.addressable = true (TODO)
				cand.addressable = true
				c.methodsAndFields(obj.Type(), cand.addressable, cand.imp, func(newCand candidate) {
					newCand.pathInvokeMask = cand.pathInvokeMask
					newCand.path = path
					c.deepState.enqueue(newCand)
				})
			}
		}
	}
}

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

	var aliasName string
	cand.name, aliasName = gopDeepCandName(cand, c.pkg.GetTypes())
	if item, err := c.item(ctx, *cand); err == nil {
		c.items = append(c.items, item)
		if aliasName != cand.name {
			ac := *cand
			ac.name = aliasName
			ac.score += 0.0001
			if item, err := c.item(ctx, ac); err == nil {
				c.items = append(c.items, item)
			}
		}
	} else if false && goxls.DbgCompletion {
		log.Println("gopCompleter.addCandidate item:", err)
		log.SingleStack()
	}
}

// deepCandName produces the full candidate name including any
// ancestor objects. For example, "foo.bar().baz" for candidate "baz".
func gopDeepCandName(cand *candidate, this *types.Package) (name string, alias string) {
	totalLen := len(cand.obj.Name())
	totalLen2 := totalLen
	for i, obj := range cand.path {
		n := len(obj.Name()) + 1
		totalLen += n
		totalLen2 += n
		if cand.pathInvokeMask&(1<<uint16(i)) > 0 {
			totalLen += 2
		}
	}

	var buf strings.Builder
	buf.Grow(totalLen)
	var buf2 strings.Builder
	buf.Grow(totalLen2)

	for i, obj := range cand.path {
		buf.WriteString(obj.Name())
		buf2.WriteString(gopStyleName(obj, this))
		if cand.pathInvokeMask&(1<<uint16(i)) > 0 {
			buf.WriteByte('(')
			buf.WriteByte(')')
		}
		buf.WriteByte('.')
		buf2.WriteByte('.')
	}

	buf.WriteString(cand.obj.Name())
	buf2.WriteString(gopStyleName(cand.obj, this))

	return buf.String(), buf2.String()
}

func gopStyleName(obj types.Object, this *types.Package) (name string) {
	name = obj.Name()
	if isFunc(obj) {
		if pkg := obj.Pkg(); pkg != nil && pkg != this {
			if strings.ToUpper(name) != name {
				if c := name[0]; c >= 'A' && c <= 'Z' {
					return string(rune(c)-('A'-'a')) + name[1:]
				}
			}
		}
	}
	return
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
