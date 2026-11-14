---
type: fix
---

Fans: a single hot CPU reading no longer triggers the CPU emergency (all fans at 100 %); the curve and the emergency use the median of the last three readings (6 s), only a reading 10 °C over the limit counts at once. Refs [#306](https://github.com/ugreen-nas-control/ugreen-nas-control/issues/306). Reported and measured by [snipah](https://github.com/snipah).
