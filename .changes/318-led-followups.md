---
type: fix
---

Use the actual default-route interface for gateway neighbour checks, including Unraid's shim-br0; keep the physical uplink for network LED speed/activity. Beep conflict errors no longer claim root is needed. First-time LED enabling initializes status mode and a steady white power LED only when settings are absent or empty, preserving existing and imported settings. Reported by [deiviest](https://github.com/deiviest) (#332, #333, #334).
