---
type: fix
---

IT8613E watchdog: a clean restart of the fan service no longer ends the protection (a marker restores the BIOS-armed timer), and a feed never lets the timer read 0 in between. `power-on-ac set` works from an unrecognised value. The page says that a hard kill of the service resets the NAS, and shows a stale, non-counting value as not counting.
