#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
go_bin=${GO:-go}
cd "$root"

"$go_bin" test -count=1 -run '^(TestOpenCreatesVersionedWALSchema|TestMigrationIsIdempotentAndChecksumProtected|TestOnlineBackupHasIntegrityManifestAndRejectsTampering|TestOnlineBackupRunsWhileWritesContinue|TestRestoreStagesAndUpgradesBeforeAtomicReplace)$' ./internal/storage
