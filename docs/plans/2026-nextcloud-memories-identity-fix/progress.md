# Progress

- [x] Step 1: Verify and document the source identity contract
- [x] Step 2: Replace fragile metadata indexing with asset-identity-first indexing
- [x] Step 3: Preserve album membership and synthetic tags through duplicate collapse
- [ ] Step 4: Improve diagnostics and fail-closed options
- [ ] Step 5: Update docs and plan tracking

## Notes

- 2026-06-13: Live test migration showed a severe metadata join failure. The importer reported `Collected 58998 indexed files from /Photos` but logged `92368` `Nextcloud Memories asset is not indexed; importing without source metadata` warnings during the same run.
- 2026-06-13: Live test migration only created one synthetic `immich-go/src/nextcloud-memories/album/...` tag assignment and barely attempted album creation, which is consistent with widespread metadata join failure.
- 2026-06-13: Current hypothesis is that the metadata index uses a basename/path approximation that does not match the actual discovered DAV/local file paths reliably enough for live Memories libraries.
- 2026-06-13: Separate follow-up concern: duplicate upload paths appear not to merge album membership and synthetic tags robustly when multiple source occurrences collapse onto a single destination asset.
- 2026-06-13: Confirmed from `internal/nextcloud` that the importer can model Memories source identity around `fileid`, with `image/info/<fileid>` returning the canonical `filename` plus album clusters.
- 2026-06-13: Reworked `adapters/nextcloudmemories/metadata.go` so the metadata index is keyed by Memories asset ID first and learns canonical paths from `image/info` hydration.
- 2026-06-13: Reworked shared duplicate handling in `app/upload/run.go` so `AlreadyProcessed`, `SameOnServer`, and `BetterOnServer` merge albums and tags onto the canonical Immich asset before issuing album/tag updates.
- 2026-06-13: Short shared-impact audit suggests the duplicate-membership fix is generally correct for other sources too, because the shared upload pipeline is used by `from-folder`, `from-google-photos`, and `from-immich`, all of which can attach albums and/or tags before deduplication.

## PR Reasoning Notes

- The Memories identity fix is source-specific: `from-nextcloud-memories` was relying on a fragile path/basename join that failed badly on a real library.
- The duplicate-membership fix is shared pipeline correctness work: the old logic could drop relationship metadata whenever a source asset matched an already-known local or server asset.
- This shared fix is expected to benefit any importer that sets `asset.Albums` or `asset.Tags` before entering `app/upload`, including `from-folder`, `from-google-photos`, and `from-immich`.
- The change does not broaden upload selection or alter duplicate detection rules; it preserves metadata that should already have been applied to the canonical destination asset.
