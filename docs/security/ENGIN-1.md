# ENGIN-1: code scanning reconciliation

The October 16, 2026 baseline has 113 open gosec alerts on main commit
`6d1ecacb22f4ea95a54789f6071c9267205f37b9`: 57 errors and 56 warnings across
21 files. These are scanner classifications; GitHub did not assign security
severity levels. [The alert inventory](ENGIN-1-alerts.csv) records every baseline
ID, original location, disposition, and GitHub link.

| Rule | Baseline | Resolution |
| --- | ---: | --- |
| G104 | 56 | Propagate errors from synchronous operations; log background and notification failures; aggregate cleanup errors. |
| G204 | 33 | Review intentional execution at each call site. Quote desktop shell/AppleScript/PowerShell literals and retain argument boundaries. |
| G304 | 8 | Document intentional reads of explicitly selected/configured files at each call site. Reject traversal in named plugin operations. |
| G301 | 8 | Create per-user config, plugin, database, and autostart directories with mode 0700. |
| G306 | 6 | Create private files with mode 0600. Write config/plugin updates atomically, including replacements of older permissive files. |
| G703 | 2 | Replace predictable upgrade staging with an owner-only temporary directory and checked rollback. Document helper discovery as an intentional Stat-only probe. |

The subprocess annotations cover operations central to this application:
installing a selected catalog entry, checking a detected executable, running
opt-in local detection plugins, and opening fixed desktop utilities. They are
per-call `#nosec` comments with reasons, recognized by both standalone gosec and
golangci-lint. They do not authorize executing commands from untrusted issue
text or other unrelated inputs. File-read annotations cover explicit CLI file
arguments and the configured local catalog/plugin/bundled-executable locations.
There are no new global rule exclusions or bulk GitHub dismissals. Existing
global G306/G703 and detector-strategy G204 lint exclusions are removed.

Regression tests cover staging symlinks, adjacent backup preservation, executable
permissions, partial-download cleanup, rejected plugin traversal, private and
atomic config writes, failed config serialization, failed JSON output, Linux
autostart error propagation, and literal desktop/script arguments. AppleScript
and PowerShell literal round trips run on their native CI platforms.

Reproduce the security check with Go 1.25 or newer:

```sh
go install github.com/securego/gosec/v2/cmd/gosec@v2.29.0
gosec -fmt sarif -out results.sarif ./...
```

CI uses this pinned scanner version, fails on findings, and uploads SARIF even
when the scan fails. It preserves the existing SARIF tool GUID, display name,
and analysis category for continued tracking of the same baseline alerts.
A green job previously did not establish remediation because
the scanner used `-no-fail`.

Default-branch GitHub alerts require a new scan after the remediation PR is
merged. A clean local or PR scan establishes the candidate's status; it does not
by itself close the baseline alerts on main.
