---
type: fix
---
Diag and the boot start ask the LED controller up to three times before they call the probe failed, and diag shows the tool's real reason instead of "failed: -" (DXP8800 Ultra report). Thanks to [halloleo](https://github.com/halloleo).
