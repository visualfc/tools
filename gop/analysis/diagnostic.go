// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package analysis

import (
	"golang.org/x/tools/go/analysis"
)

// A Diagnostic is a message associated with a source location or range.
//
// An Analyzer may return a variety of diagnostics; the optional Category,
// which should be a constant, may be used to classify them.
// It is primarily intended to make it easy to look up documentation.
//
// If End is provided, the diagnostic is specified to apply to the range between
// Pos and End.
type Diagnostic = analysis.Diagnostic

// RelatedInformation contains information related to a diagnostic.
// For example, a diagnostic that flags duplicated declarations of a
// variable may include one RelatedInformation per existing
// declaration.
type RelatedInformation = analysis.RelatedInformation

// A SuggestedFix is a code change associated with a Diagnostic that a
// user can choose to apply to their code. Usually the SuggestedFix is
// meant to fix the issue flagged by the diagnostic.
//
// TextEdits for a SuggestedFix should not overlap,
// nor contain edits for other packages.
type SuggestedFix = analysis.SuggestedFix

// A TextEdit represents the replacement of the code between Pos and End with the new text.
// Each TextEdit should apply to a single file. End should not be earlier in the file than Pos.
type TextEdit = analysis.TextEdit
