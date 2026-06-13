# Progress

- [ ] Step 1: Verify and document the source identity contract
- [ ] Step 2: Replace fragile metadata indexing with asset-identity-first indexing
- [ ] Step 3: Preserve album membership and synthetic tags through duplicate collapse
- [ ] Step 4: Improve diagnostics and fail-closed options
- [ ] Step 5: Update docs and plan tracking

## Notes

- 2026-06-13: Live test migration showed a severe metadata join failure. The importer reported `Collected 58998 indexed files from /Photos` but logged `92368` `Nextcloud Memories asset is not indexed; importing without source metadata` warnings during the same run.
- 2026-06-13: Live test migration only created one synthetic `immich-go/src/nextcloud-memories/album/...` tag assignment and barely attempted album creation, which is consistent with widespread metadata join failure.
- 2026-06-13: Current hypothesis is that the metadata index uses a basename/path approximation that does not match the actual discovered DAV/local file paths reliably enough for live Memories libraries.
- 2026-06-13: Separate follow-up concern: duplicate upload paths appear not to merge album membership and synthetic tags robustly when multiple source occurrences collapse onto a single destination asset.
