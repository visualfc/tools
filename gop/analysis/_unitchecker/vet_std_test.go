// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unitchecker_test

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/tools/gop/analysis/passes/asmdecl"
	"golang.org/x/tools/gop/analysis/passes/assign"
	"golang.org/x/tools/gop/analysis/passes/atomic"
	"golang.org/x/tools/gop/analysis/passes/bools"
	"golang.org/x/tools/gop/analysis/passes/buildtag"
	"golang.org/x/tools/gop/analysis/passes/cgocall"
	"golang.org/x/tools/gop/analysis/passes/composite"
	"golang.org/x/tools/gop/analysis/passes/copylock"
	"golang.org/x/tools/gop/analysis/passes/directive"
	"golang.org/x/tools/gop/analysis/passes/errorsas"
	"golang.org/x/tools/gop/analysis/passes/framepointer"
	"golang.org/x/tools/gop/analysis/passes/httpresponse"
	"golang.org/x/tools/gop/analysis/passes/ifaceassert"
	"golang.org/x/tools/gop/analysis/passes/loopclosure"
	"golang.org/x/tools/gop/analysis/passes/lostcancel"
	"golang.org/x/tools/gop/analysis/passes/nilfunc"
	"golang.org/x/tools/gop/analysis/passes/printf"
	"golang.org/x/tools/gop/analysis/passes/shift"
	"golang.org/x/tools/gop/analysis/passes/sigchanyzer"
	"golang.org/x/tools/gop/analysis/passes/stdmethods"
	"golang.org/x/tools/gop/analysis/passes/stringintconv"
	"golang.org/x/tools/gop/analysis/passes/structtag"
	"golang.org/x/tools/gop/analysis/passes/testinggoroutine"
	"golang.org/x/tools/gop/analysis/passes/tests"
	"golang.org/x/tools/gop/analysis/passes/timeformat"
	"golang.org/x/tools/gop/analysis/passes/unmarshal"
	"golang.org/x/tools/gop/analysis/passes/unreachable"
	"golang.org/x/tools/gop/analysis/passes/unusedresult"
	"golang.org/x/tools/gop/analysis/unitchecker"
)

// vet is the entrypoint of this executable when ENTRYPOINT=vet.
// Keep consistent with the actual vet in GOROOT/src/cmd/vet/main.go.
func vet() {
	unitchecker.Main(
		asmdecl.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		directive.Analyzer,
		errorsas.Analyzer,
		framepointer.Analyzer,
		httpresponse.Analyzer,
		ifaceassert.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		printf.Analyzer,
		shift.Analyzer,
		sigchanyzer.Analyzer,
		stdmethods.Analyzer,
		stringintconv.Analyzer,
		structtag.Analyzer,
		tests.Analyzer,
		testinggoroutine.Analyzer,
		timeformat.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		// unsafeptr.Analyzer, // currently reports findings in runtime
		unusedresult.Analyzer,
	)
}

// TestVetStdlib runs the same analyzers as the actual vet over the
// standard library, using go vet and unitchecker, to ensure that
// there are no findings.
func TestVetStdlib(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}
	if version := runtime.Version(); !strings.HasPrefix(version, "devel") {
		t.Skipf("This test is only wanted on development branches where code can be easily fixed. Skipping because runtime.Version=%q.", version)
	}

	cmd := exec.Command("go", "vet", "-vettool="+os.Args[0], "std")
	cmd.Env = append(os.Environ(), "ENTRYPOINT=vet")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("go vet std failed (%v):\n%s", err, out)
	}
}
