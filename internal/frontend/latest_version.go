// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package frontend

import (
	"context"

	"github.com/khulnasoft-lab/godep/internal"
	"github.com/khulnasoft-lab/godep/internal/frontend/versions"
	"github.com/khulnasoft-lab/godep/internal/log"
	"github.com/khulnasoft-lab/godep/internal/middleware/stats"
)

// GetLatestInfo returns various pieces of information about the latest
// versions of a unit and module:
//   - The linkable form of the minor version of the unit.
//   - The latest module path and the full unit path of any major version found given the
//     fullPath and the modulePath.
//
// It returns empty strings on error.
// It is intended to be used as an argument to middleware.LatestVersions.
func (s *Server) GetLatestInfo(ctx context.Context, unitPath, modulePath string, latestUnitMeta *internal.UnitMeta) internal.LatestInfo {
	defer stats.Elapsed(ctx, "GetLatestInfo")()

	// It is okay to use a different DataSource (DB connection) than the rest of the
	// request, because this makes self-contained calls on the DB.
	ds := s.getDataSource(ctx)

	latest, err := ds.GetLatestInfo(ctx, unitPath, modulePath, latestUnitMeta)
	if err != nil {
		log.Errorf(ctx, "Server.GetLatestInfo: %v", err)
	} else {
		latest.MinorVersion = versions.LinkVersion(latest.MinorModulePath, latest.MinorVersion, latest.MinorVersion)
	}
	return latest
}
