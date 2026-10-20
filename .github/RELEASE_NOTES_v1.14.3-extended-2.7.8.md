# sing-box-extended v1.14.3-extended-2.7.8

Updates the core to upstream `shtorm-7/sing-box-extended v1.14.3-extended-2.7.3` (sing-box 1.14.3).

Preserved extensions:

- WireGuard endpoint IPC methods used for live peer management.
- AmneziaWG 3.1 `random_trailers` and `disable_cookies` options in wireguard and warp amnezia settings.
- Runtime removal of inbounds, endpoints and services (`Remove(tag)`), so configuration changes can be applied to a running instance without restarting the core.
- Hot-added inbounds, endpoints and services are started immediately through their own lifecycle scope and can be removed later without restarting the core.
- OpenVPN support through the managed `sing-openvpn` fork.
- Rule-set file cleanup and strict binary-stream validation.

The WireGuard dependency is `github.com/shtorm-7/wireguard-go v0.0.8-extended-1.6.2`, which keeps the correct handshake wire format with random trailers.

New upstream options in this release: `disable_path_mtu_discovery` for TrustTunnel inbound and outbound, and `handshake_timeout` plus `use_l4_proxy` for the MASQUE outbound.

The module path remains `github.com/sagernet/sing-box`. Go consumers can select this release with:

```go
replace github.com/sagernet/sing-box => github.com/deposist/sing-box-extended v1.14.3-extended-2.7.8
```

Go does not inherit dependency replacement directives. Copy the replacement directives from this release's `go.mod` into the consuming module, including the WireGuard, sing, tailscale and OpenVPN replacements.
