// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/khulnasoft-lab/godep/internal/frontend/versions"
	"github.com/khulnasoft-lab/godep/internal/postgres"
)

func TestFrontendVersionsPage(t *testing.T) {
	defer postgres.ResetTestDB(testDB, t)
	ctx := context.Background()
	processVersions(ctx, t, testModules)

	const modulePath = "example.com/symbols"
	for _, test := range []struct {
		name, pkgPath string
		want          []*versions.VersionList
	}{
		{"versions page symbols - one version all symbols", modulePath, versionsPageSymbols},
		{"versions page hello - multi GOOS", modulePath + "/hello", versionsPageHello},
		{"versions page - test symbol signature is different for different build context", modulePath + "/multigoos", versionsPageMultiGoos},
		{"versions page - test symbol is introduced at different versions for different build context and changes across versions",
			modulePath + "/duplicate", versionsPageMultiGoosDuplicates},
	} {
		t.Run(test.name, func(t *testing.T) {
			urlPath := fmt.Sprintf("/%s?tab=versions&content=json", test.pkgPath)
			body := getFrontendPage(t, urlPath)
			var got versions.VersionsDetails
			if err := json.Unmarshal([]byte(body), &got); err != nil {
				t.Fatalf("json.Unmarshal: %v", err)
			}
			if diff := cmp.Diff(test.want, got.ThisModule, cmpopts.IgnoreUnexported(versions.Symbol{})); diff != "" {
				t.Errorf("mismatch (-want, got):\n%s", diff)
			}
		})
	}
}
