---
type: fix
---

Power on after AC loss: IT8613E models (DXP2800, DXP4800, DXP4800 Plus/Pro, DXP6800 Pro, DXP8800 Plus) write the saved choice back at every boot, because the BIOS set its own default again at each start. Reported by [manilx](https://github.com/manilx).
