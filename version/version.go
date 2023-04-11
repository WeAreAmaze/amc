// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package version

import (
	"fmt"
)

var (
	// Following vars are injected through the build flags (see Makefile)
	GitCommit string
	GitBranch string
	GitTag    string
)

// see https://calver.org
const (
	VersionMajor       = 2022  // Major version component of the current release
	VersionMinor       = 99    // Minor version component of the current release
	VersionMicro       = 99    // Patch version component of the current release
	VersionModifier    = "dev" // Modifier component of the current release
	VersionKeyCreated  = "AmcVersionCreated"
	VersionKeyFinished = "AmcVersionFinished"
)

func withModifier(vsn string) string {
	if !isStable() {
		vsn += "-" + VersionModifier
	}
	return vsn
}

func isStable() bool {
	return VersionModifier == "stable"
}

func isRelease() bool {
	return isStable() || VersionModifier == "alpha" || VersionModifier == "beta"
}

// Version holds the textual version string.
var Version = func() string {
	return fmt.Sprintf("%d.%02d.%d", VersionMajor, VersionMinor, VersionMicro)
}()

// VersionWithMeta holds the textual version string including the metadata.
var VersionWithMeta = func() string {
	v := Version
	if VersionModifier != "" {
		v += "-" + VersionModifier
	}
	return v
}()
