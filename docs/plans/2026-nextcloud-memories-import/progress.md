# Nextcloud Memories Import Progress

## Current Status

**Phase**: Source discovery, base asset enumeration, and upload-path optimization

**Last Updated**: 2026-06-01

**Summary**:

The command shape, scope model, guardrails, and draft user-facing documentation have been outlined. The internal Nextcloud client layer uses `gowebdav` for DAV plus custom `net/http` for non-DAV APIs. Discovery is implemented, and the hidden command can now enumerate supported media from the selected Memories timeline roots and hand those assets to the existing upload pipeline. Upload preparation now reuses a single cached source read for checksum calculation and upload streaming, which removes an avoidable second fetch for non-local sources such as WebDAV. Public upload docs remain unchanged until more of the source behavior is shipped.

---

## Step Tracking

- [x] Define the command direction and scope model
  - Import should target the configured Memories library, not arbitrary Nextcloud folders
  - Auto-discovery is the default behavior

- [x] Define escape hatches and guardrails
  - Drafted root-limiting, discovery-only, and indexing override options
  - Drafted failure behavior for missing app and invalid configuration

- [x] Draft user-facing command documentation
  - Added a planned command reference and examples in `user-docs-draft.md`
  - Kept public docs unchanged because the command is not implemented

- [x] Scaffold command UX and validation
  - Added a hidden `from-nextcloud-memories` command shell with alias `from-nc-memories`
  - Added source flag registration, normalization, and validation for the planned UX contract
  - Added tests for command metadata, flag validation, and explicit not-implemented failure paths
  - Added a human-readable scaffold summary so the CLI UX can be reviewed before discovery exists

- [x] Choose dependency strategy and add client layer
  - Added `internal/nextcloud` as the home for the source-side client abstraction
  - Chose `gowebdav` for DAV access and custom `net/http` for OCS and Memories endpoints
  - Added tests for URL normalization, request construction, and OCS header behavior

- [x] Implement source authentication and discovery
  - Added OCS and Memories discovery helpers in `internal/nextcloud`
  - Wired `--discover-only` to validate DAV access and print detected source configuration
  - Added tests for discovery responses, empty timeline roots, and command output

- [x] Implement file enumeration from configured Memories roots
  - Added a read-only DAV-backed filesystem wrapper in `internal/nextcloud`
  - Added `Browse()` for the Nextcloud Memories adapter so imports now emit uploadable assets
  - Filters supported media, ignores common banned/useless files, and deduplicates overlapping selected roots

- [x] Reuse cached source reads across checksum and upload
  - Centralized asset cache creation so checksum and upload share the same cached representation
  - Avoids a second source fetch for non-local readers such as Nextcloud WebDAV
  - Added regression tests that assert a single source open across checksum and upload flows

- [ ] Implement metadata and album mapping

- [x] Add focused tests for discovery and base enumeration
  - Added DAV filesystem tests for path handling and file reads
  - Added adapter browse tests for media filtering and overlapping-root deduplication

- [ ] Promote draft docs into public docs after implementation ships

---

## Decisions

### 2026-06-01: Public Docs Stay Accurate

**Decision**: Do not add `from-nextcloud-memories` to `docs/commands/upload.md` yet.

**Rationale**:

- The command is not implemented
- The project guidelines require user-facing docs to track shipped behavior
- Draft UX docs belong in `docs/plans/` until code and tests exist

### 2026-06-01: No Positional Source Path

**Decision**: The proposed command should not take a positional `<source-path>`.

**Rationale**:

- A Memories migration should import the configured Memories library
- Requiring manual folder selection would make this a generic Nextcloud importer instead
- Existing `from-folder` already covers the generic import case

### 2026-06-01: `timeline_path` Defines Scope

**Decision**: Default import scope should be derived from the effective Memories `timeline_path` configuration.

**Rationale**:

- `timeline_path` is the actual Memories library scope
- `folders_path` is a UI navigation root, not the library definition
- Importing outside `timeline_path` would violate user expectations

### 2026-06-01: Command Stays Hidden Until Functional

**Decision**: Register the scaffold command as hidden while discovery and browsing are still missing.

**Rationale**:

- It allows iterative work on command UX in the real CLI surface
- It avoids advertising a non-functional source in normal command listings
- Public docs can remain accurate until the command becomes usable

### 2026-06-01: Use `gowebdav` Only For DAV

**Decision**: Use `github.com/studio-b12/gowebdav` for DAV operations, while keeping OCS and Memories calls in `internal/nextcloud` with custom `net/http` code.

**Rationale**:

- DAV is the stable and reusable part of the source integration
- OCS and Memories endpoints still need importer-specific request shaping and error handling
- This avoids overcommitting to a niche Nextcloud client dependency that still would not cover Memories properly
- It keeps the dependency surface small and aligns with the project's preference for minimal external libraries