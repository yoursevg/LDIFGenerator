# LDIFGenerator

Production-oriented MVP for generating large LDIF files for LDAP load testing. The backend is written in Go, writes records with a streaming `bufio.Writer`, parses LDAP schema LDIF, resolves `SUP` inheritance, validates `MUST`/`MAY`, and exposes a Wails + React UI.

## Architecture

- `internal/schema`: LDIF schema parser, model, aliases, multiline unfolding, `SUP` resolver.
- `internal/ldif`: record model, LDIF encoder, line folding, base64 output, streaming writer.
- `internal/generator`: config, fake data registry, record generator, tree plans, relationships.
- `internal/validation`: DN and schema-aware record validation.
- `internal/app`: GUI-facing service with load/generate/progress/cancel methods.
- `frontend`: React/TypeScript Wails UI.

The extension point for new fake data is `generator.AttributeGenerator`. Register a new generator in `NewFakeRegistry()` by attribute name or alias.

## Run CLI

```bash
go run ./cmd/ldifgenerator -schema /path/to/schema.ldif -config /path/to/config.json
```

## Run GUI

```bash
cd frontend
npm install
cd ..
wails dev
```

For a production desktop build:

```bash
cd frontend && npm install && npm run build
cd ..
wails build
```

## Test

```bash
go test ./...
```

## Current MVP

- Reads one or more schema LDIF files.
- Parses `attributeTypes` and `objectClasses`, including aliases like `NAME ( 'cn' 'commonName' )`.
- Supports folded LDIF schema values.
- Resolves inherited `MUST`/`MAY` attributes through `SUP`.
- Generates users, privileged users, groups, computers, service accounts and OU containers.
- Writes LDIF streaming to disk.
- Uses `privUser` for privileged users and `serviceUser` for service accounts by default.
- Generates group `member`, user `memberOf`, nested groups and manager links.
- Supports deterministic seed, batch progress, cancellation and a generation report.
- Validates required/allowed attributes in strict mode.

## Notes

Schema files, generated LDIF files, and local test configs may contain environment-specific data and are intentionally not checked in. Real output is produced by the generator and may contain 100k or 1M+ entries without keeping all records in memory.
