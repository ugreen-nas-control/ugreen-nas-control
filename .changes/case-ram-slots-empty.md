---
type: fix
---

Case view: after a reboot the Inside view showed the RAM slots as empty (and no maximum LAN speed) until something else had read the memory; the plugin page started its tools without a search path, so dmidecode and ethtool were not found.
