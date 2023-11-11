// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package analysisflags

import (
	"fmt"
	"net/url"

	"golang.org/x/tools/gop/analysis"
)

// ResolveURL resolves the URL field for a Diagnostic from an Analyzer
// and returns the URL. See Diagnostic.URL for details.
func ResolveURL(a analysis.IAnalyzer, d analysis.Diagnostic) (string, error) {
	aURL := analysis.URL(a)
	if d.URL == "" && d.Category == "" && aURL == "" {
		return "", nil // do nothing
	}
	raw := d.URL
	if d.URL == "" && d.Category != "" {
		raw = "#" + d.Category
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid Diagnostic.URL %q: %s", raw, err)
	}
	base, err := url.Parse(aURL)
	if err != nil {
		return "", fmt.Errorf("invalid Analyzer.URL %q: %s", aURL, err)
	}
	return base.ResolveReference(u).String(), nil
}
