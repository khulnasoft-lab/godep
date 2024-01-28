// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// deps_test tests dependencies of cmd/godep are kept to a limited set.
package deps_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/khulnasoft-lab/godep/internal/testenv"
)

// non-test packages are allowed to depend on licensecheck and safehtml, x/ repos, and markdown.
var allowedModDeps = map[string]bool{
	"github.com/google/licensecheck": true,
	"github.com/google/safehtml":     true,
	"golang.org/x/mod":               true,
	"github.com/khulnasoft-lab/godep/net":               true,
	"github.com/khulnasoft-lab/godep":           true,
	"golang.org/x/sync":              true,
	"golang.org/x/text":              true,
	"golang.org/x/tools":             true,
	"rsc.io/markdown":                true,
}

// test packages are also allowed to depend on go-cmp
var additionalAllowedTestModDeps = map[string]bool{
	"github.com/google/go-cmp": true,
}

func TestCmdGodepDeps(t *testing.T) {
	testenv.MustHaveExecPath(t, "go")

	// First, list all dependencies of godep.
	out, err := exec.Command("go", "list", "-e", "-deps", "github.com/khulnasoft-lab/godep/cmd/godep").Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			t.Fatalf("running go list -test -deps on package github.com/khulnasoft-lab/godep/cmd/godep:\n%s", ee.Stderr)
		}
		t.Fatalf("running go list -test -deps on package github.com/khulnasoft-lab/godep/cmd/godep: %v", err)

	}
	pkgs := strings.Fields(string(out))
	for _, pkg := range pkgs {
		// Filter to only the dependencies that are in the godep module.
		if !strings.HasPrefix(pkg, "github.com/khulnasoft-lab/godep") {
			continue
		}

		// Get the test module deps and check them against allowedTestModDeps.
		out, err := exec.Command("go", "list", "-e", "-deps", "-test", "-f", "{{if .Module}}{{.Module.Path}}{{end}}", pkg).Output()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
				t.Fatalf("running go list -test -deps on package %s:\n%s", pkg, ee.Stderr)
			}
			t.Fatalf("running go list -test -deps on package %s: %v", pkg, err)
		}
		testmodules := strings.Fields(string(out))
		for _, m := range testmodules {
			if !(allowedModDeps[m] || additionalAllowedTestModDeps[m]) {
				t.Fatalf("disallowed test module dependency %q found through %q", m, pkg)
			}
		}

		// Get the module deps and check them against allowedModDeps
		out, err = exec.Command("go", "list", "-e", "-deps", "-f", "{{if .Module}}{{.Module.Path}}{{end}}", pkg).Output()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
				t.Fatalf("running go list -deps on package %s:\n%s", pkg, ee.Stderr)
			}
			t.Fatalf("running go list -deps on package %s: %v", pkg, err)
		}
		modules := strings.Fields(string(out))
		for _, m := range modules {
			if !allowedModDeps[m] {
				t.Fatalf("disallowed module dependency %q found through %q", m, pkg)
			}
		}
	}
}
