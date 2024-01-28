// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package versions

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/khulnasoft-lab/godep/internal"
	"github.com/khulnasoft-lab/godep/internal/osv"
	"github.com/khulnasoft-lab/godep/internal/stdlib"
	"github.com/khulnasoft-lab/godep/internal/testing/fakedatasource"
	"github.com/khulnasoft-lab/godep/internal/testing/sample"
	"github.com/khulnasoft-lab/godep/internal/version"
	"github.com/khulnasoft-lab/godep/internal/vuln"
)

var (
	modulePath1 = "test.com/module"
	modulePath2 = "test.com/module/v2"
)

func sampleModule(modulePath, version string, versionType version.Type, packages ...*internal.Unit) *internal.Module {
	if len(packages) == 0 {
		return sample.Module(modulePath, version, sample.Suffix)
	}
	m := sample.Module(modulePath, version)
	for _, p := range packages {
		sample.AddUnit(m, p)
	}
	return m
}

func versionSummaries(path string, versions []string, isStdlib bool, linkify func(path, version string) string) []*VersionSummary {
	vs := make([]*VersionSummary, len(versions))
	for i, version := range versions {
		semver := version
		if isStdlib {
			semver = stdlib.VersionForTag(version)
		}
		vs[i] = &VersionSummary{
			Version:    version,
			Link:       linkify(path, version),
			CommitTime: absoluteTime(sample.CommitTime),
			IsMinor:    isMinor(semver),
		}
	}
	return vs
}

func TestFetchPackageVersionsDetails(t *testing.T) {
	var (
		v2Path = "test.com/module/v2/foo"
		v1Path = "test.com/module/foo"
	)

	pkg1 := &internal.Unit{
		UnitMeta: *sample.UnitMeta(
			modulePath1+"/"+sample.Suffix,
			modulePath1,
			"v1.2.1",
			sample.Suffix,
			true),
		IsRedistributable: true,
		Documentation:     []*internal.Documentation{sample.Doc},
	}
	pkg2 := &internal.Unit{
		UnitMeta: *sample.UnitMeta(
			modulePath2+"/"+sample.Suffix,
			modulePath2,
			"v1.2.1-alpha.1",
			sample.Suffix,
			true),
		IsRedistributable: true,
		Documentation:     []*internal.Documentation{sample.Doc},
	}
	nethttpPkg := &internal.Unit{
		UnitMeta: *sample.UnitMeta(
			"net/http",
			"std",
			"v1.12.5",
			"http",
			true),
		IsRedistributable: true,
		Documentation:     []*internal.Documentation{sample.Doc},
	}
	makeList := func(pkgPath, modulePath, major string, versions []string, isStdlib, incompatible bool) *VersionList {
		return &VersionList{
			VersionListKey: VersionListKey{ModulePath: modulePath, Major: major, Incompatible: incompatible},
			Versions: versionSummaries(pkgPath, versions, isStdlib, func(path, version string) string {
				return ConstructUnitURL(pkgPath, modulePath, version)
			}),
		}
	}

	vulnFixedVersion := "1.2.3"
	vulnEntry := &osv.Entry{
		ID:      "GO-1999-0001",
		Summary: "summary",
		Details: "description",
		Affected: []osv.Affected{{
			Module: osv.Module{
				Path: modulePath1,
			},
			Ranges: []osv.Range{{
				Type:   osv.RangeTypeSemver,
				Events: []osv.RangeEvent{{Introduced: "1.2.0"}, {Fixed: vulnFixedVersion}},
			}},
			EcosystemSpecific: osv.EcosystemSpecific{
				Packages: []osv.Package{{
					Path: v1Path,
				}},
			},
		}},
	}
	vc, err := vuln.NewInMemoryClient([]*osv.Entry{vulnEntry})
	if err != nil {
		t.Fatal(err)
	}

	// Named bool values, for readability of call sites below.
	const (
		stdlib, notStdlib        = true, false
		compatible, incompatible = false, true
	)

	for _, tc := range []struct {
		name        string
		pkg         *internal.Unit
		modules     []*internal.Module
		wantDetails *VersionsDetails
	}{
		{
			name: "want stdlib versions",
			pkg:  nethttpPkg,
			modules: []*internal.Module{
				sampleModule("std", "v1.22.0-rc.1", version.TypePseudo, nethttpPkg),
				sampleModule("std", "v1.21.0", version.TypeRelease, nethttpPkg),
				sampleModule("std", "v1.20.0", version.TypeRelease, nethttpPkg),
				sampleModule("std", "v1.12.5", version.TypeRelease, nethttpPkg),
				sampleModule("std", "v1.11.6", version.TypeRelease, nethttpPkg),
			},
			wantDetails: &VersionsDetails{
				ThisModule: []*VersionList{
					makeList(
						"net/http", "std", "go1",
						[]string{
							"go1.22rc1",
							"go1.21.0",
							"go1.20",
							"go1.12.5",
							"go1.11.6",
						},
						stdlib, compatible,
					),
				},
			},
		},
		{
			name: "want v1 first",
			pkg:  pkg1,
			modules: []*internal.Module{
				sampleModule(modulePath1, "v0.0.0-20140414041502-3c2ca4d52544", version.TypePseudo, pkg2),
				sampleModule(modulePath1, "v1.2.3", version.TypeRelease, pkg1),
				sampleModule(modulePath1, "v2.1.0+incompatible", version.TypeRelease, pkg1),
				sampleModule(modulePath2, "v2.0.0", version.TypeRelease, pkg2),
				sampleModule(modulePath1, "v1.3.0", version.TypeRelease, pkg1),
				sampleModule(modulePath1, "v1.2.1", version.TypeRelease, pkg1),
				sampleModule(modulePath2, "v2.2.1-alpha.1", version.TypePrerelease, pkg2),
				sampleModule("test.com", "v1.2.1", version.TypeRelease, pkg1),
			},
			wantDetails: &VersionsDetails{
				ThisModule: []*VersionList{
					func() *VersionList {
						vl := makeList(v1Path, modulePath1, "v1", []string{"v1.3.0", "v1.2.3", "v1.2.1"}, notStdlib, compatible)
						vl.Versions[2].Vulns = []vuln.Vuln{{
							ID:      vulnEntry.ID,
							Details: vulnEntry.Summary,
						}}
						return vl
					}(),
				},
				IncompatibleModules: []*VersionList{
					makeList(v1Path, modulePath1, "v2", []string{"v2.1.0+incompatible"}, notStdlib, incompatible),
				},
				OtherModules: []string{"test.com", modulePath2},
			},
		},
		{
			name: "want v2 first",
			pkg:  pkg2,
			modules: []*internal.Module{
				sampleModule(modulePath1, "v0.0.0-20140414041502-3c2ca4d52544", version.TypePseudo, pkg1),
				sampleModule(modulePath1, "v1.2.1", version.TypeRelease, pkg1),
				sampleModule(modulePath1, "v1.2.3", version.TypeRelease, pkg1),
				sampleModule(modulePath1, "v2.1.0+incompatible", version.TypeRelease, pkg1),
				sampleModule(modulePath2, "v2.0.0", version.TypeRelease, pkg2),
				sampleModule(modulePath2, "v2.2.1-alpha.1", version.TypePrerelease, pkg2),
			},
			wantDetails: &VersionsDetails{
				ThisModule: []*VersionList{
					makeList(v2Path, modulePath2, "v2", []string{"v2.2.1-alpha.1", "v2.0.0"}, notStdlib, compatible),
				},
				OtherModules: []string{modulePath1},
			},
		},
		{
			name: "want only pseudo",
			pkg:  pkg2,
			modules: []*internal.Module{
				sampleModule(modulePath1, "v0.0.0-20140414041501-3c2ca4d52544", version.TypePseudo, pkg2),
				sampleModule(modulePath1, "v0.0.0-20140414041502-4c2ca4d52544", version.TypePseudo, pkg2),
			},
			wantDetails: &VersionsDetails{
				OtherModules: []string{modulePath1},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			fds := fakedatasource.New()

			for _, v := range tc.modules {
				fds.MustInsertModule(ctx, v)
			}

			got, err := FetchVersionsDetails(ctx, fds, &tc.pkg.UnitMeta, vc)
			if err != nil {
				t.Fatalf("FetchVersionsDetails(ctx, db, %q, %q): %v", tc.pkg.Path, tc.pkg.ModulePath, err)
			}
			for _, vl := range tc.wantDetails.ThisModule {
				for _, v := range vl.Versions {
					v.CommitTime = absoluteTime(tc.modules[0].CommitTime)
				}
			}
			if diff := cmp.Diff(tc.wantDetails, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPathInVersion(t *testing.T) {
	tests := []struct {
		v1Path, modulePath, want string
	}{
		{"foo.com/bar/baz", "foo.com/bar", "foo.com/bar/baz"},
		{"foo.com/bar/baz", "foo.com/bar/v2", "foo.com/bar/v2/baz"},
		{"foo.com/bar/baz", "foo.com/v3", "foo.com/v3/bar/baz"},
		{"foo.com/bar/baz", "foo.com/bar/baz/v3", "foo.com/bar/baz/v3"},
	}

	for _, test := range tests {
		mi := sample.ModuleInfo(test.modulePath, sample.VersionString)
		if got := pathInVersion(test.v1Path, mi); got != test.want {
			t.Errorf("pathInVersion(%q, ModuleInfo{...ModulePath:%q}) = %s, want %v",
				test.v1Path, mi.ModulePath, got, test.want)
		}
	}
}

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		version, want string
	}{
		{"v1.2.3", "v1.2.3"},
		{"v2.0.0", "v2.0.0"},
		{"v1.2.3-alpha.1", "v1.2.3-alpha.1"},
		{"v1.0.0-20190311183353-d8887717615a", "v1.0.0-...-d888771"},
		{"v1.2.3-pre.0.20190311183353-d8887717615a", "v1.2.3-pre.0...-d888771"},
		{"v1.2.4-0.20190311183353-d8887717615a", "v1.2.4-0...-d888771"},
		{"v1.0.0-20190311183353-d88877", "v1.0.0-...-d88877"},
		{"v1.0.0-longprereleasestring", "v1.0.0-longprereleases..."},
		{"v1.0.0-pre-release.0.20200420093620-87861123c523", "v1.0.0-pre-rele...-8786112"},
		{"v0.0.0-20190101-123456789012", "v0.0.0-20190101-123456..."}, // prelease version that looks like pseudoversion
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			if got := formatVersion(test.version); got != test.want {
				t.Errorf("formatVersion(%q) = %q, want %q", test.version, got, test.want)
			}
		})
	}
}

func TestPseudoVersionBase(t *testing.T) {
	tests := []struct {
		version, want string
	}{
		{"v1.0.0-20190311183353-d8887717615a", "v1.0.0-"},
		{"v1.2.3-pre.0.20190311183353-d8887717615a", "v1.2.3-pre.0"},
		{"v1.2.4-0.20190311183353-d8887717615a", "v1.2.4-0"},
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			if got := pseudoVersionBase(test.version); got != test.want {
				t.Errorf("pseudoVersionBase(%q) = %q, want %q", test.version, got, test.want)
			}
		})
	}
}

func TestIsMinor(t *testing.T) {
	for _, test := range []struct {
		version string
		want    bool
	}{
		{"v0.5.0", true},
		{"v1.0.0-pre", false},
		{"v1.0.0", true},
		{"v1.0.1", false},
		{"v2.0.0+incompatible", false},
		{"v1.0.0-20190311183353-d8887717615a", false},
		{"v1.2.3-pre.0.20190311183353-d8887717615a", false},
		{"v1.2.4-0.20190311183353-d8887717615a", false},
	} {
		t.Run(test.version, func(t *testing.T) {
			if got := isMinor(test.version); got != test.want {
				t.Errorf("isMinor(%q) = %t, want %t", test.version, got, test.want)
			}
		})
	}
}

func TestDisplayVersion(t *testing.T) {
	for _, test := range []struct {
		name             string
		fullPath         string
		requestedVersion string
		resolvedVersion  string
		want             string
	}{
		{
			"std @ master",
			stdlib.ModulePath,
			version.Master,
			stdlib.TestMasterVersion,
			"master (89fb59e)",
		},
		{
			"std @ latest is master",
			stdlib.ModulePath,
			version.Latest,
			stdlib.TestMasterVersion,
			"master (89fb59e)",
		},
		{
			"std @ latest is go1.16",
			stdlib.ModulePath,
			version.Latest,
			"v1.16.0",
			"go1.16",
		},
		{
			"std @ go1.16",
			stdlib.ModulePath,
			"v1.16.0",
			"v1.16.0",
			"go1.16",
		},
		{
			"std @ dev.fuzz",
			stdlib.ModulePath,
			"dev.fuzz",
			stdlib.TestMasterVersion,
			"dev.fuzz (89fb59e)",
		},
		{
			"github.com path @ latest is v1.5.2",
			sample.ModulePath,
			version.Latest,
			"v1.5.2",
			"v1.5.2",
		},
		{
			"github.com path @ master is v1.5.2",
			sample.ModulePath,
			version.Master,
			"v1.5.2",
			"v1.5.2",
		},
		{
			"github.com path @ v1.5.2",
			sample.ModulePath,
			"v1.5.2",
			"v1.5.2",
			"v1.5.2",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := DisplayVersion(test.fullPath, test.requestedVersion, test.resolvedVersion); got != test.want {
				t.Errorf("DisplayVersion(%q, %q, %q) = %q, want %q",
					test.fullPath, test.requestedVersion, test.resolvedVersion, got, test.want)
			}
		})
	}
}

func TestLinkVersion(t *testing.T) {
	for _, test := range []struct {
		name             string
		fullPath         string
		requestedVersion string
		resolvedVersion  string
		want             string
	}{
		{
			"std @ master",
			stdlib.ModulePath,
			version.Master,
			stdlib.TestMasterVersion,
			version.Master,
		},
		{
			"std @ latest is master",
			stdlib.ModulePath,
			version.Latest,
			stdlib.TestMasterVersion,
			version.Master,
		},
		{
			"std @ latest is go1.16",
			stdlib.ModulePath,
			version.Latest,
			"v1.16.0",
			"go1.16",
		},
		{
			"std @ go1.16",
			stdlib.ModulePath,
			"v1.16.0",
			"v1.16.0",
			"go1.16",
		},
		{
			"std @ dev.fuzz",
			stdlib.ModulePath,
			"dev.fuzz",
			stdlib.TestMasterVersion,
			"dev.fuzz",
		},
		{
			"github.com path @ latest is v1.5.2",
			sample.ModulePath,
			version.Latest,
			"v1.5.2",
			"v1.5.2",
		},
		{
			"github.com path @ master is v1.5.2",
			sample.ModulePath,
			version.Master,
			"v1.5.2",
			"v1.5.2",
		},
		{
			"github.com path @ v1.5.2",
			sample.ModulePath,
			"v1.5.2",
			"v1.5.2",
			"v1.5.2",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := LinkVersion(test.fullPath, test.requestedVersion, test.resolvedVersion); got != test.want {
				t.Errorf("LinkVersion(%q, %q, %q) = %q, want %q",
					test.fullPath, test.requestedVersion, test.resolvedVersion, got, test.want)
			}
		})
	}
}
