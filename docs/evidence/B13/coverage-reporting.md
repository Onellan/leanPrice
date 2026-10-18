# B13 — Coverage, reporting and administration
**Status:** reporting slice partially implemented; expansion/account/retention work open  
**Candidate SHA:** `77795c4301089f6ddd0b1bd5ab5f010ebf277c55`  
**Authority:** FR-020/040; AC-O12; source, privacy and security authority.

## Implemented locally
- Bounded keyset-paged CSV export (`internal/storage/export.go`) and streaming authenticated route (`internal/httpui/catalog_handlers.go`). It selects validated exact approved/enabled public standard/special history, caps rows, excludes source URL/secrets, escapes formula-like free text and sets private/no-store headers.
- Product detail provides a bounded observed price range only for comparable validated in-stock observations, labels sample size and avoids savings claims.
- `go test -count=1 ./internal/storage ./internal/httpui`: PASS for filtering, row/field limits, formula escaping, authentication, headers and sampled range semantics.

## Not implemented / blocker
Additional adapters require B00 approval. Retention/deletion controls and optional household accounts are not implemented; account access requires explicit product scope plus privacy review. B13 remains partial.
