# sing-box-extended v1.14.0-extended-2.7.4

Fixed a shutdown hang in the MTProxy inbound. The listener now closes before the proxy waits for active connections to finish, so stopping or restarting the core releases the MTProxy port. Existing MTProxy configurations and Telegram links remain valid.
