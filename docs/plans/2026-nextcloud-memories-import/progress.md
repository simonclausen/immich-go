# Nextcloud Memories Import Progress

## Current Status

**Phase**: Hidden command supports source metadata and album mapping

**Last Updated**: 2026-06-02

**Summary**:

The command shape, scope model, guardrails, and draft user-facing documentation have been outlined. The internal Nextcloud client layer uses `gowebdav` for basic DAV access plus custom `net/http` for non-DAV APIs. Discovery is implemented, and the hidden command can enumerate supported media from the selected Memories timeline roots and hand those assets to the existing upload pipeline. Enumeration uses DAV directory walking. The command also supports reading file contents from a local synced directory instead of WebDAV while still using server discovery for Memories scope validation. Upload preparation reuses a single cached source read for checksum calculation and upload streaming, which removes an avoidable second fetch for non-local sources such as WebDAV. Source-side metadata mapping is now wired through Memories day and image-info APIs, including capture date, GPS, description, rating, favorite, archive state, tags, and album membership. Public upload docs remain unchanged until the hidden command is promoted.

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
  - Added a hidden `from-nextcloud-memories` command shell
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

- [x] Drop the unverified WebDAV `SEARCH` path
  - Removed the untested recursive DAV `SEARCH` implementation and its adapter fast path
  - Enumeration now uses the standard DAV directory walk only
  - Kept the local synced directory optimization for fast file reads during migration

- [x] Allow local synced directory as preferred file source
  - Added `--nextcloud-local-dir` to prefer a local sync directory when opening asset contents while keeping Memories as the source of truth
  - Enumeration still comes from Nextcloud discovery and DAV-backed browsing, so local files are an optimization path rather than an override
  - Added focused tests for the layered local-first file source behavior

### 2026-06-02: No Short Alias

**Decision**: Keep the command name as `from-nextcloud-memories` without a `from-nc-memories` alias.

**Rationale**:

- The full name is explicit enough for copy-paste use
- Shared examples and support instructions are clearer with one canonical spelling
- The local copy optimization is the more valuable usability improvement

- [x] Reuse cached source reads across checksum and upload
  - Centralized asset cache creation so checksum and upload share the same cached representation
  - Avoids a second source fetch for non-local readers such as Nextcloud WebDAV
  - Added regression tests that assert a single source open across checksum and upload flows

- [x] Implement metadata and album mapping
  - Added Memories timeline and per-file image-info API helpers in `internal/nextcloud`
  - Built a source metadata index keyed by the file paths returned from Memories image-info
  - Mapped capture date, GPS, description, rating, favorite, archive state, tags, and album membership onto imported assets
  - Added partial-index detection so DAV-enumerated files without Memories metadata warn and continue by default, with `--require-indexed` available for fail-closed imports
  - Shared albums are renamed with an owner suffix when needed to avoid album title collisions in Immich

- [x] Add focused tests for discovery and base enumeration
  - Added DAV filesystem tests for path handling and file reads
  - Added adapter browse tests for media filtering and overlapping-root deduplication

- [ ] Promote draft docs into public docs after implementation ships

### 2026-06-02: Album Membership Uses Source Metadata

**Decision**: Resolve album membership from per-file Memories image-info responses instead of building a separate album-first traversal.

**Rationale**:

- It keeps DAV enumeration as the single source of asset discovery
- It avoids a second source of truth for file selection and duplicate handling
- The shared upload pipeline already recreates albums once asset membership is populated
- It makes partial indexing visible immediately because missing image-info means missing source metadata

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