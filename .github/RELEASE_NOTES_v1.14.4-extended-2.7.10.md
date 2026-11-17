# sing-box-extended v1.14.4-extended-2.7.10

Updates the sing-box base to `SagerNet/sing-box v1.14.4` while preserving the
extended features and compatibility fixes used by s-ui-x-extended.

Preserved extensions and fixes:

- WireGuard endpoint IPC methods and AmneziaWG options used for live peer management.
- Runtime addition and removal of inbounds, endpoints and services with scoped lifecycle cleanup.
- Nested outbound group selection, interruption and DNS loop safeguards.
- Tailnet DNS routing priorities and cancellable pause waits.
- OpenVPN support through the managed `sing-openvpn` fork.
- Rule-set file cleanup and strict binary-stream validation.
- Sudoku stream reset on receive-queue overflow.

Modified dependencies are pinned to the same managed forks:

- `github.com/deposist/sing v0.9.7-extended-1.2.2`
- `github.com/deposist/tailscale v1.102.1-sing-box-1.14-mod.5-extended-1.0.4`
- `github.com/deposist/sing-openvpn v0.0.1-deposist.2`

The module path remains `github.com/sagernet/sing-box`. Go consumers can select
this release with:

```go
replace github.com/sagernet/sing-box => github.com/deposist/sing-box-extended v1.14.4-extended-2.7.10
```

Go does not inherit dependency replacement directives. Copy the replacement
directives from this release's `go.mod` into the consuming module so that the
same managed dependency revisions are used in release builds.
