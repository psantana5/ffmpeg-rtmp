package wrapper

// If the wrapper crashes, the workload MUST continue.
// If we are unsure, DO LESS.
// If something is not reversible, DO NOT TOUCH IT.
// This is governance, not execution.

// This file is intentionally minimal.
// Lifecycle observation happens in internal/observe.
// No retries. No restarts. No policy.
