---
type: fix
---

EC reads and beeper transactions share ug-fand's /run/ug-ec.lock, and ug-fand is recognized as a competing fan controller while LED control remains available. Install and update honor fan control being off before probing. Reported by [deiviest](https://github.com/deiviest) (#318); hardware confirmation of coexistence is still needed.
