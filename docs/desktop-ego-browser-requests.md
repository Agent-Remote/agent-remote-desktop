# Ego Browser request control

The desktop uses two versioned `--desktop-control` methods for the active
ego-browser request ledger:

- `ego_browser.requests` (`read`, `user` credential) lists one exact binding's
  active requests. The result is bounded to 128 items and contains only IDs,
  generations, sequence numbers, message type, payload byte count, status and
  creation time. Browser payloads and credentials never cross the bridge.
- `ego_browser.request.cancel` (`write`, `user` credential,
  `exact_request` confirmation) cancels one reviewed ledger entry. The request
  freezes the binding generation, request generation and sequence. The Rust CLI
  refreshes the binding and request ledger immediately before submitting the
  cancellation, so stale desktop state fails closed.

Both methods require an exact `expected_context` containing the configuration
home, server URL and active device. The CLI owns user-token loading and API
calls. A successful mutation returns an operation receipt whose `operation_id`
is the desktop request ID and includes the resulting request projection.

The Go adapter validates capability metadata, context equality, bounded fields,
and the receipt before exposing data to the UI. The Ego Browser page lets the
user inspect requests per binding and submit a cancellation from a frozen row;
the existing managed terminal commands remain available for broader lifecycle
operations such as setup, claim, pause and resume.
