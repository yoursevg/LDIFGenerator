# LDIFGenerator

Production-oriented CLI for generating large LDIF files for LDAP load testing. It is written in Go, writes records with a streaming `bufio.Writer`, parses LDAP schema LDIF, resolves `SUP` inheritance, and validates `MUST`/`MAY`.

## Architecture

- `internal/schema`: LDIF schema parser, model, aliases, multiline unfolding, `SUP` resolver.
- `internal/ldif`: record model, LDIF encoder, line folding, base64 output, streaming writer.
- `internal/generator`: config, fake data registry, record generator, tree plans, relationships.
- `internal/ldapimport`: LDIF chunking and phased `ldapadd` runner.
- `internal/validation`: DN and schema-aware record validation.
- `cmd/ldifgenerator`: CLI entrypoint.
- `cmd/ldapbulkadd`: helper CLI for concurrent phased imports through `ldapadd`.
- `cmd/schemaaudit`: helper CLI for inspecting parsed schema counts and warnings.

The extension point for new fake data is `generator.AttributeGenerator`. Register a new generator in `NewFakeRegistry()` by attribute name or alias.

## Run CLI

```bash
go run ./cmd/ldifgenerator -schema /path/to/schema.ldif -config /path/to/config.json
```

`-schema` accepts a comma-separated list of schema files and/or directories. Directories are scanned recursively for `.ldif`, `.schema`, and `.conf` files, then parsed in deterministic path order.

## Concurrent ldapadd

`ldapbulkadd` splits a generated LDIF into temporary chunks and imports them with `ldapadd` in dependency-friendly phases: containers first, regular entries second, groups with `member` values last. Regular entry chunks run concurrently; group chunks run serially by default so nested groups keep generation order.

```bash
go run ./cmd/ldapbulkadd -file generated.ldif -jobs 8 -chunk-records 5000 -- \
  -x -H ldap://localhost:389 -D cn=admin,dc=example,dc=com -w secret -c
```

Use `-group-jobs` only when group records are independent enough for parallel adds in your server setup.

To only prepare phased LDIF chunks:

```bash
go run ./cmd/ldapbulkadd -file generated.ldif -split-only -workdir ./chunks
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
- Generates fallback values from LDAP attribute syntax OIDs where possible, including Numeric String, Postal Address, DN, Boolean, Integer, Generalized Time, Telephone Number, IA5/Directory String and common binary syntaxes.
- Supports deterministic seed, batch progress, cancellation and a generation report.
- Validates required/allowed attributes in strict mode.

## Notes

Schema files, generated LDIF files, and local test configs may contain environment-specific data and are intentionally not checked in. Real output is produced by the generator and may contain 100k or 1M+ entries without keeping all records in memory.
