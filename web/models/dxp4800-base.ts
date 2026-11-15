import type { Model } from './schema';

export default {
    id: 'dxp4800-base',
    name: 'DXP4800',
    series: 'DXP',
    dmiPattern: '^DXP4800$',
    fixtureDmi: 'DXP4800',
    cpu: 'Intel N100',
    ram: { type: 'DDR5', mt: 4800 },
    bays: 4,
    m2Slots: 2,
    m2: { slots: ['m2-1', 'm2-2'], pcieSlot: false },
    lan: { maxMbit: [2500, 2500] },
    led: 'bays',
    caseView: 'full',
    caseVariant: 'rear4800',
    fan: {
        chip: 'ITE IT8613E',
        fans: { fan3: 'case' },
        kind: 'probe',
        watchdog: {
            status: 'expected',
            note: 'Expected, from the vendor driver and the kernel it87_wdt: the plugin only feeds a timer it sees counting, never arms it, and yields to it87_wdt. Not confirmed on this hardware.'
        }
    },
    verified: {
        leds: {
            status: 'community',
            by: 'rogvalinger-ops (issue 299)',
            version: '5.6.0',
            date: '2026-11-12',
            ref: 'https://github.com/ugreen-nas-control/ugreen-nas-control/issues/299',
            note: 'Reported working on 5.6.0 (#299): LED probe ok at 0x3a, 6 LEDs. Also reported working by users of the DXP4800 family.'
        },
        fans: {
            status: 'community',
            by: 'rogvalinger-ops (issue 299)',
            version: '5.6.0',
            date: '2026-11-12',
            ref: 'https://github.com/ugreen-nas-control/ugreen-nas-control/issues/299',
            note: "Reported working on 5.6.0 (#299): IT8613 found, the fan probe confirmed header 3 as the case fan, which spun at 602 rpm under plugin control. No cpu fan output, as in UGREEN's own driver (header 3 only). Same IT8613E as the DXP6800 Pro."
        },
        case: {
            status: 'community',
            by: 'rogvalinger-ops (issue 299)',
            version: '5.6.0',
            date: '2026-11-12',
            ref: 'https://github.com/ugreen-nas-control/ugreen-nas-control/issues/299',
            note: "Reported working on 5.6.0 (#299): two 2.5G LAN ports (Intel I226-V), drawn on the rear. Inside view after UGREEN's manual and a photo of the open panel: two M.2 slots at the left, one SO-DIMM socket; which SSD sits in which M.2 slot is assumed (the two NVMe drives sit on root ports 00:1d.2 and 00:1d.0)."
        },
        sensors: {
            status: 'community',
            by: 'rogvalinger-ops (issue 299)',
            version: '5.6.0',
            date: '2026-11-12',
            ref: 'https://github.com/ugreen-nas-control/ugreen-nas-control/issues/299',
            note: 'Reported working on 5.6.0 (#299): cpu temperature ok.'
        },
        panel: {
            status: 'partial',
            by: 'rogvalinger-ops (issue 299)',
            version: '5.6.0',
            date: '2026-11-12',
            ref: 'https://github.com/ugreen-nas-control/ugreen-nas-control/issues/299',
            note: 'Reported working on 5.6.0 (#299): the panel tables are present and the form says the front panel works. No button test or beeper test was reported, so the buttons stay unconfirmed.'
        }
    },
    sources: ['https://nascompares.com/2025/11/14/new-ugreen-dxp4800-pro-and-dxp4800s-nas-revealed/'],
    note: 'Four bays, two M.2 slots, two 2.5G LAN ports. Fan control after a fan probe.',
    shots: false
} satisfies Model;
