// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package inspector

import (
	"github.com/goplus/gop/ast"
)

// An Inspector provides methods for inspecting
// (traversing) the syntax trees of a package.
type Inspector struct {
	events []event
}

// New returns an Inspector for the specified syntax trees.
func New(files []*ast.File) *Inspector {
	return &Inspector{traverse(files)}
}

// An event represents a push or a pop
// of an ast.Node during a traversal.
type event struct {
	node  ast.Node
	typ   uint64 // typeOf(node) on push event, or union of typ strictly between push and pop events on pop events
	index int    // index of corresponding push or pop event
}

// TODO: Experiment with storing only the second word of event.node (unsafe.Pointer).
// Type can be recovered from the sole bit in typ.

// Preorder visits all the nodes of the files supplied to New in
// depth-first order. It calls f(n) for each node n before it visits
// n's children.
//
// The complete traversal sequence is determined by ast.Inspect.
// The types argument, if non-empty, enables type-based filtering of
// events. The function f is called only for nodes whose type
// matches an element of the types slice.
func (in *Inspector) Preorder(types []ast.Node, f func(ast.Node)) {
	// Because it avoids postorder calls to f, and the pruning
	// check, Preorder is almost twice as fast as Nodes. The two
	// features seem to contribute similar slowdowns (~1.4x each).

	mask := maskOf(types)
	for i := 0; i < len(in.events); {
		ev := in.events[i]
		if ev.index > i {
			// push
			if ev.typ&mask != 0 {
				f(ev.node)
			}
			pop := ev.index
			if in.events[pop].typ&mask == 0 {
				// Subtrees do not contain types: skip them and pop.
				i = pop + 1
				continue
			}
		}
		i++
	}
}

// traverse builds the table of events representing a traversal.
func traverse(files []*ast.File) []event {
	// Preallocate approximate number of events
	// based on source file extent.
	// This makes traverse faster by 4x (!).
	var extent int
	for _, f := range files {
		extent += int(f.End() - f.Pos())
	}
	// This estimate is based on the net/http package.
	capacity := extent * 33 / 100
	if capacity > 1e6 {
		capacity = 1e6 // impose some reasonable maximum
	}
	events := make([]event, 0, capacity)

	var stack []event
	stack = append(stack, event{}) // include an extra event so file nodes have a parent
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if n != nil {
				// push
				ev := event{
					node:  n,
					typ:   0,           // temporarily used to accumulate type bits of subtree
					index: len(events), // push event temporarily holds own index
				}
				stack = append(stack, ev)
				events = append(events, ev)
			} else {
				// pop
				top := len(stack) - 1
				ev := stack[top]
				typ := typeOf(ev.node)
				push := ev.index
				parent := top - 1

				events[push].typ = typ            // set type of push
				stack[parent].typ |= typ | ev.typ // parent's typ contains push and pop's typs.
				events[push].index = len(events)  // make push refer to pop

				stack = stack[:top]
				events = append(events, ev)
			}
			return true
		})
	}

	return events
}
