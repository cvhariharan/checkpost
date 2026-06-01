package migrations

import "embed"

// Files holds the database migrations embedded into the binary. They are run
// at startup via the golang-migrate iofs source driver.
//
//go:embed postgres
var Files embed.FS
