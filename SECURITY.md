# Security Policy

Security fixes are provided for the latest release and the current `main` branch.

## Reporting a vulnerability

Do not report vulnerabilities in a public issue.

Use [GitHub Security Advisories](https://github.com/sibukixxx/insight/security/advisories/new) (preferred), or email takada@techvit.me. Include the affected version or commit, reproduction steps or a proof of concept, and the expected impact when possible.

We aim to acknowledge reports within 48 hours and will share progress while investigating. Please do not disclose details before a fix is released.

## Project-specific considerations

- API keys entered in the UI or with `--api-key` are held in process memory only. They are never written to SQLite or disk.
- Analysis sends the required document text to the configured LLM API. Review the provider's data policy before processing confidential or personal data.
- The default production build excludes demo data. Before distributing any binary, database, or archive, verify that it contains no customer projects or interview content.
