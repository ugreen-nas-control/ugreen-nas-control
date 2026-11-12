---
type: fix
---

MQTT: the IP address of a port is found when the port belongs to a bond that belongs to a bridge (eth0, bond0, br0, the usual Unraid layout); before, only the first step up was tried and `ip` stayed empty.
