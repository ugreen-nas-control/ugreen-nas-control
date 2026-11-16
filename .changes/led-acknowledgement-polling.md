---
type: fix
---

LED commands poll for a delayed controller acknowledgement before resending, while still reporting commands that exhaust their retries. Addresses a timing weakness investigated after [halloleo](https://github.com/halloleo)'s DXP8800 Ultra report ([#340](https://github.com/ugreen-nas-control/ugreen-nas-control/issues/340)); confirmation on that hardware is still needed.
