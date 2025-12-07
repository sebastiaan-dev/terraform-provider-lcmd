# Coding Agent Instructions

- Any Go source you touch must end up formatted exactly as `gofmt` would produce, including tab indentation and canonical spacing.
- Keep changes surgical and limit edits to the scope of the request so history remains easy to follow.
- Preserve deterministic behavior in the Terraform provider, especially for artifact naming, upload flows, and cached build logic.
- When behavior changes, ensure related documentation or user-facing notes capture the update.
