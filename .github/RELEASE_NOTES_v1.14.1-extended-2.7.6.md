# sing-box-extended v1.14.1-extended-2.7.6

Updates the core to upstream `shtorm-7/sing-box-extended v1.14.1-extended-2.7.2` (sing-box 1.14.1).

Preserves the WireGuard endpoint IPC methods used for live peer management, AmneziaWG 3.1 `random_trailers` and `disable_cookies` options, rule-set file cleanup and strict binary-stream validation. The WireGuard dependency remains `deposist/wireguard-go v0.0.5-extended-1.6.2`, which includes the random-trailers handshake fix.

The module path remains `github.com/sagernet/sing-box`. Go consumers can select this release with:

```go
replace github.com/sagernet/sing-box => github.com/deposist/sing-box-extended v1.14.1-extended-2.7.6
```

Go does not inherit dependency replacement directives. Copy the replacement directives from this release's `go.mod` into the consuming module, including the WireGuard and OpenVPN replacements.
