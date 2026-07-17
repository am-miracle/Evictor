# Security policy

Please do not report vulnerabilities in public issues. Send a private report
to the repository maintainers through the security contact configured for the
hosting repository (GitHub Security Advisories when enabled), including a
description, impact, reproduction steps, and a safe contact method.

We will acknowledge reports within seven days and coordinate a fix and public
disclosure timeline with the reporter.

## Historical credential audit

The repository history was scanned for private-key blocks, cloud access-key
patterns, tokens, and credential-bearing connection URLs. No real credential
was found. Earlier setup commits did contain the placeholder `evictor/evictor`
development password in examples; it is not a production credential, but any
deployment that reused it must rotate it immediately. Git history is permanent and should be treated as public.
