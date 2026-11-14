---
type: fix
---

LED discovery checks native I801 and DesignWare adapters for a valid MCU response, preferring the HT32 ACPI adapter, before considering the existing memory fallbacks. An empty first adapter no longer hides a working one, and an Intel DesignWare adapter keeps its model's write protocol.
