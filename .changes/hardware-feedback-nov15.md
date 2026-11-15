---
type: fix
---

Case view: place a detected DXP4800 Plus 2.5G interface on its physical socket even when the 10G interface is missing. Thanks to [bram-van](https://github.com/bram-van) (#323). Clarify the confirmed physical DIMM1/DIMM2 labels versus the unconfirmed software-to-socket mapping from [ivolution](https://github.com/ivolution) (#297). LEDs: check brightness, colour and timing when reconciling activity blinks, so an incorrect blink no longer bypasses correction. Investigated after [Killerkomando](https://github.com/Killerkomando)'s iDX6011 Pro night-mode report (#324); the cause of brightness changing on the hardware still needs confirmation.
