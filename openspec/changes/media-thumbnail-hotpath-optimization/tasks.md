# Tasks

- [x] Baseline the current hot path (`newFetchSession`, transport/client creation, cache hit copies, `buildCompositeForURL`) and identify the smallest safe optimization points.
- [x] Implement performance-only changes that reuse client/transport safely and reduce unnecessary buffer copies without changing validation, pinning, redaction, cache semantics, or HTTP responses.
- [x] Add focused verification for cache-hit behavior and thumbnail hot-path safety/performance assumptions, then close the stream with explicit evidence.
