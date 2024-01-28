// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package worker

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/khulnasoft-lab/godep/internal"
	"github.com/khulnasoft-lab/godep/internal/derrors"
	"github.com/khulnasoft-lab/godep/internal/licenses"
	"github.com/khulnasoft-lab/godep/internal/postgres"
	"github.com/khulnasoft-lab/godep/internal/proxy/proxytest"
	"github.com/khulnasoft-lab/godep/internal/source"
	"github.com/khulnasoft-lab/godep/internal/testing/sample"
	"github.com/khulnasoft-lab/godep/internal/testing/testhelper"
)

func TestReFetch(t *testing.T) {
	ctx := context.Background()

	// This test checks that re-fetching a version will cause its data to be
	// overwritten.  This is achieved by fetching against two different versions
	// of the (fake) proxy, though in reality the most likely cause of changes to
	// a version is updates to our data model or fetch logic.
	defer postgres.ResetTestDB(testDB, t)

	var (
		modulePath = sample.ModulePath
		version    = sample.VersionString
		pkgFoo     = sample.ModulePath + "/foo"
		foo        = map[string]string{
			"foo/foo.go": "// Package foo\npackage foo\n\nconst Foo = 42",
			"README.md":  "This is a readme",
			"LICENSE":    testhelper.MITLicense,
		}
		pkgBar = sample.ModulePath + "/bar"
		foobar = map[string]string{
			"LICENSE":       testhelper.MITLicense,
			"bar/README.md": "This is a readme",
			"bar/bar.go":    "// Package bar\npackage bar\n\nconst Bar = 21",
			"foo/foo.go":    "// Package foo\npackage foo\n\nconst Foo = 42",
		}
	)

	// First fetch and insert a version containing package foo, and verify that
	// foo can be retrieved.
	proxyClient, teardownProxy := proxytest.SetupTestClient(t, []*proxytest.Module{
		{
			ModulePath: modulePath,
			Version:    version,
			Files:      foo,
		},
	})
	defer teardownProxy()
	sourceClient := source.NewClient(http.DefaultClient)
	f := &Fetcher{proxyClient, sourceClient, testDB, nil, nil, ""}
	if _, _, err := f.FetchAndUpdateState(ctx, sample.ModulePath, version, testAppVersion); err != nil {
		t.Fatalf("FetchAndUpdateState(%q, %q): %v", sample.ModulePath, version, err)
	}

	if _, err := testDB.GetUnitMeta(ctx, pkgFoo, internal.UnknownModulePath, version); err != nil {
		t.Error(err)
	}

	// Now re-fetch and verify that contents were overwritten.
	proxyClient, teardownProxy = proxytest.SetupTestClient(t, []*proxytest.Module{
		{
			ModulePath: sample.ModulePath,
			Version:    version,
			Files:      foobar,
		},
	})
	defer teardownProxy()

	f = &Fetcher{proxyClient, sourceClient, testDB, nil, nil, ""}
	if _, _, err := f.FetchAndUpdateState(ctx, sample.ModulePath, version, testAppVersion); err != nil {
		t.Fatalf("FetchAndUpdateState(%q, %q): %v", modulePath, version, err)
	}
	want := &internal.Unit{
		UnitMeta: internal.UnitMeta{
			ModuleInfo: internal.ModuleInfo{
				ModulePath:        sample.ModulePath,
				Version:           version,
				CommitTime:        time.Date(2019, 1, 30, 0, 0, 0, 0, time.UTC),
				IsRedistributable: true,
				SourceInfo:        source.NewGitHubInfo("https://"+sample.ModulePath, "", sample.VersionString),
			},
			Path: sample.ModulePath + "/bar",
			Name: "bar",
		},
		IsRedistributable: true,
		Licenses: []*licenses.Metadata{
			{Types: []string{"MIT"}, FilePath: "LICENSE"},
		},
		Readme: &internal.Readme{
			Filepath: "bar/README.md",
			Contents: "This is a readme",
		},
		Documentation: []*internal.Documentation{{
			Synopsis: "Package bar",
			GOOS:     "linux",
			GOARCH:   "amd64",
		}},
		Subdirectories: []*internal.PackageMeta{
			{
				Path:              "github.com/valid/module_name/bar",
				Name:              "bar",
				Synopsis:          "Package bar",
				IsRedistributable: true,
				Licenses:          sample.LicenseMetadata(),
			},
		},
		SymbolHistory: map[string]string{"Bar": "v1.0.0"},
	}
	got, err := testDB.GetUnitMeta(ctx, pkgBar, internal.UnknownModulePath, version)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want.UnitMeta, *got,
		cmp.AllowUnexported(source.Info{}),
		cmpopts.IgnoreFields(licenses.Metadata{}, "Coverage"),
		cmpopts.IgnoreFields(internal.UnitMeta{}, "HasGoMod")); diff != "" {
		t.Fatalf("testDB.GetUnitMeta(ctx, %q, %q) mismatch (-want +got):\n%s", want.ModulePath, want.Version, diff)
	}

	gotPkg, err := testDB.GetUnit(ctx, got, internal.WithMain, internal.BuildContext{})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, gotPkg,
		cmp.AllowUnexported(source.Info{}),
		cmpopts.IgnoreFields(internal.Unit{}, "Documentation", "BuildContexts"),
		cmpopts.IgnoreFields(licenses.Metadata{}, "Coverage"),
		cmpopts.IgnoreFields(internal.UnitMeta{}, "HasGoMod")); diff != "" {
		t.Errorf("mismatch on readme (-want +got):\n%s", diff)
	}
	if got, want := gotPkg.Documentation, want.Documentation; got == nil || want == nil {
		if !cmp.Equal(got, want) {
			t.Fatalf("mismatch on documentation: got: %v\nwant: %v", got, want)
		}
		return
	}

	// Now re-fetch and verify that contents were overwritten.
	proxyClient, teardownProxy = proxytest.SetupTestClient(t, []*proxytest.Module{
		{
			ModulePath: modulePath,
			Version:    version,
			Files:      foo,
		},
	})
	defer teardownProxy()
	f = &Fetcher{proxyClient, sourceClient, testDB, nil, nil, ""}
	if _, _, err := f.FetchAndUpdateState(ctx, modulePath, version, testAppVersion); !errors.Is(err, derrors.DBModuleInsertInvalid) {
		t.Fatalf("FetchAndUpdateState(%q, %q): %v", modulePath, version, err)
	}
}
