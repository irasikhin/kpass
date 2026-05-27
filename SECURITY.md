# Security Policy

## Supported Versions

kpass is pre-1.0 alpha software. Security fixes are provided for the latest
release only.

## Reporting a Vulnerability

Do not open a public issue for a vulnerability. Report security issues by
emailing the maintainer listed in the GitHub repository profile, or by using
GitHub private vulnerability reporting if it is enabled for this repository.

Please include:

- affected kpass version or commit;
- operating system and install method;
- a minimal reproduction that does not include real passwords, key files, OTP
  seeds, or database contents.

## Secret Handling Notes

kpass intentionally exposes secrets for commands such as `get --all`,
`get --field password`, `get --field otp`, `copy`, and `export`. Treat terminal
history, shell history, process listings, clipboard managers, logs, and export
files as sensitive when using those commands.

Prefer prompts or `--password-stdin` over inline `--password` in shared systems,
because inline arguments may be recorded by shells or visible to local process
inspection.
