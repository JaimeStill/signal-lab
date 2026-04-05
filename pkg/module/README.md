# pkg/module

HTTP module and router system. Provides prefix-based route mounting with composable middleware stacks.

Adapted from `~/code/herald/pkg/module/`.

## Files (Phase 1)

- `module.go` — Module wrapping prefix + handler + middleware stack
- `router.go` — Top-level router dispatching to modules by prefix
