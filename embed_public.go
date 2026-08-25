// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import "embed"

// PublicFS contains all static assets and HTML templates compiled into the binary.
// This eliminates the runtime dependency on a public/ directory on the filesystem.
//
//go:embed all:public
var PublicFS embed.FS
