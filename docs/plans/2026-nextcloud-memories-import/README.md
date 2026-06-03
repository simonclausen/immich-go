# Nextcloud Memories Import

## Goal

Define a safe, user-friendly `immich-go` command for migrating a photo library from a Nextcloud instance that uses the Memories app.

The command should feel like a true Memories migration:

1. The user points `immich-go` at a Nextcloud instance.
2. The command authenticates with the source account.
3. The command discovers the effective Memories configuration for that user.
4. The import scope is limited to the Memories library, not the whole Nextcloud filesystem.
5. Albums and supported metadata are recreated in Immich.

## Non-Goals

- Not implementing a generic Nextcloud WebDAV importer for arbitrary folders
- Not migrating people or face assignments
- Not migrating album collaborators, shares, or comments
- Not exposing a public command in shipped docs before the feature exists
- Not changing existing upload command behavior for current sources

## Success Criteria

- The proposed command surface is explicit and consistent with existing `upload` sub-commands
- The source scope is auto-discovered from Memories configuration by default
- Escape hatches are available without turning the command into a generic folder crawler
- Unsupported source data is documented clearly
- Public docs remain accurate until implementation ships

## Proposed Command Surface

Command:

```bash
immich-go upload from-nextcloud-memories [options]
```

Rationale:

- `upload` already groups source-specific migrations under sub-commands
- `from-nextcloud-memories` is explicit and self-documenting
- Keeping one canonical command name makes examples and support instructions simpler

## Scope Model

The command should intentionally have no positional `<source-path>` argument.

Instead, the importer should:

1. Authenticate to Nextcloud
2. Check that the Memories app is installed and reachable
3. Read the effective user configuration
4. Resolve the configured `timeline_path` values
5. Import only files and albums that belong to that Memories library

This keeps the command aligned with user intent. If users want arbitrary folder import from local or remote storage, that is a different feature.

### Default Behavior

- Use the authenticated user's Memories configuration
- Auto-discover all configured `timeline_path` roots
- Preserve supported metadata and album membership
- Recreate albums in Immich when possible

### Escape Hatches

- Limit the run to one or more configured timeline roots
- Run discovery only and print resolved configuration
- Allow continuing with incomplete indexing when the user opts in
- Disable album recreation when the user wants a flat import
- Prefer reading file bytes from a local synced Nextcloud copy to reduce migration time

### Local Copy Optimization

When the source library is already synced locally by the Nextcloud desktop client, the importer can reuse that local copy for asset reads.

- `--nextcloud-local-dir` points to the local sync root
- Discovery, scope validation, and enumeration still come from the source Nextcloud Memories configuration
- Only file content reads prefer the local copy, falling back to WebDAV when a file is not present locally
- This preserves Memories semantics while avoiding slow remote reads during migration

### Guardrails

- Fail if the Memories app is missing or disabled
- Fail if `timeline_path` is empty, unknown, or invalid
- Warn or fail when the source library appears partially indexed
- Warn when unsupported source data will be ignored
- Reject arbitrary Nextcloud paths that are outside the configured Memories roots

## Client Strategy

Chosen approach: use `github.com/studio-b12/gowebdav` for the DAV file layer and keep OCS plus Memories requests in a custom internal HTTP client.

Why this is the right boundary for `immich-go`:

- It follows the project's dependency discipline: one mature dependency for the stable DAV layer, not a broad new client stack
- WebDAV is the part we do not want to reimplement by hand: directory listing, stat, stream reads, moves, and request transport handling are already solved well
- OCS and Memories still require application-specific logic, so a generic Nextcloud CLI or thin wrapper would not remove much importer code
- Memories APIs are not stable enough to hide behind a third-party dependency without losing control over error handling and compatibility
- `gowebdav` is better aligned with a source-side reader than newer upload-oriented libraries such as `godav`

Implication for implementation:

- `internal/nextcloud` will own Nextcloud base URL normalization, authentication, and HTTP request helpers
- DAV browsing and downloads will go through `gowebdav`
- OCS calls and Memories-specific endpoints will use `net/http` directly through the same internal client package

## Data Mapping

| Source data | Destination support | Notes |
| --- | --- | --- |
| Files | Yes | Imported from the configured Memories library |
| Capture date | Yes | From EXIF or Memories metadata |
| GPS | Yes | From EXIF or Memories metadata |
| Description | Yes | Imported when available |
| Rating | Yes | Imported when available |
| Favorite | Yes | Preserved on destination assets |
| Archived state | Yes | Preserved when detectable from Memories |
| Albums | Yes | Recreated in Immich |
| Tags | Yes | Recreated as Immich tags |
| People / faces | No | Intentionally out of scope |
| Album shares / collaborators | No | No equivalent migration target in this proposal |
| Comments | No | No destination mapping planned |
| Trash state | No | No destination mapping in current upload flow |

## Proposed Source Options

These are draft flags for discussion, not committed API.

| Option | Required | Description |
| --- | :---: | --- |
| `--nextcloud-url` | Y | Source Nextcloud base URL |
| `--nextcloud-user` | Y | Source Nextcloud username |
| `--nextcloud-password` | Y | Source password or app password |
| `--nextcloud-skip-verify-ssl` |  | Skip TLS verification for source |
| `--nextcloud-client-timeout` |  | Timeout for source API calls |
| `--discover-only` |  | Print detected Memories config and exit |
| `--timeline-root` |  | Limit import to configured Memories roots; repeatable |
| `--sync-albums` |  | Recreate Memories albums in Immich; default `true` |
| `--require-indexed` |  | Fail if files are found under selected Memories roots without matching Memories metadata |

## Open Questions

1. Should strict partial-index enforcement remain optional, or should the importer eventually surface a stronger summary/report for unindexed files?
2. Should hidden albums be imported by default even though Immich has no equivalent hidden-album feature?
3. Should discovery print only resolved roots, or also album support and indexing status?
4. Should the command accept both password and app password fields, or document app passwords as the recommended value for `--nextcloud-password`?

## Documentation Strategy

This proposal deliberately does not modify the shipped upload command docs in `docs/commands/` yet.

Reason:

- The command does not exist yet
- The project guidelines require user-facing docs to match implemented behavior
- Draft UX and example docs should live under `docs/plans/` until implementation lands

See `user-docs-draft.md` in this plan folder for the current draft of the future user-facing documentation.