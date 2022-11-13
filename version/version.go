// Copyright 2022 The AmazeChain Authors
// This file is part of the AmazeChain library.
//
// The AmazeChain library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The AmazeChain library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the AmazeChain library. If not, see <http://www.gnu.org/licenses/>.

package version

import "fmt"

var (
	MajorVersionNumber = 0
	MinorVersionNumber = 8
	RevisionNumber     = 2

	BuildNumber string
	BuildTime   string
	GoVersion   string
)

func PrintVersion() {
	fmt.Println(fmt.Sprintf("Service Version:	v%d.%d.%d build-%s", MajorVersionNumber, MinorVersionNumber, RevisionNumber, BuildNumber))
	fmt.Println(fmt.Sprintf("Git commit:		%s", BuildNumber))
	fmt.Println(fmt.Sprintf("Build Time:		%s", BuildTime))
	fmt.Println(fmt.Sprintf("Go Version:		%s", GoVersion))
}

func FormatVersion() string {
	return fmt.Sprintf("Service Version:	v%d.%d.%d build-%s", MajorVersionNumber, MinorVersionNumber, RevisionNumber, BuildNumber)
}

func Version() string {
	return fmt.Sprintf("v%d.%d.%d:build-%s", MajorVersionNumber, MinorVersionNumber, RevisionNumber, BuildNumber)
}
