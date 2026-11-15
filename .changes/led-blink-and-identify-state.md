---
type: fix
---

LEDs: preserve brightness when blinking starts on the DXP4800 GT, and restore Identify mode, colour and brightness before handing the bay back to the status daemon. A failed restoration keeps its saved state for retry.
