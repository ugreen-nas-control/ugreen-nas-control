import type { Model } from './schema';

export default {
    id: 'dx4600',
    name: 'DX4600',
    series: 'DX',
    dmiPattern: '^DX4600$',
    fixtureDmi: 'DX4600',
    cpu: 'Intel Celeron N5105',
    ram: null,
    bays: 4,
    m2Slots: null,
    m2: null,
    lan: null,
    led: 'bays',
    caseView: 'front-rear',
    caseVariant: 'rear4600',
    fan: null,
    verified: {
        leds: {
            status: 'community',
            by: 'sbbbsbbb',
            ref: 'https://github.com/miskcoo/ugreen_leds_controller/pull/132',
            note: 'The upstream CLI was tested on a non-Pro DX4600 with DSM 7.2.1, kernel 5.10.55+: I801 at 00:1f.4 (8086:4da3), MCU at 0x3a, valid power/network/disk1-4 status checksums, and visually confirmed on/off, colour, brightness and blink. Not tested with this plugin. Physical disk-to-bay LED order remains unconfirmed.'
        },
        fans: { status: 'unsupported' },
        case: {
            status: 'expected',
            note: 'Front and rear use the existing DX4600-family drawing, not compared on the non-Pro. No inside view until the RAM and M.2 layout is established. Bay order remains unconfirmed.'
        },
        sensors: { status: 'expected' },
        panel: { status: 'expected', note: "Buttons from UGREEN's DX4600-family driver; not checked on the non-Pro. The driver has no beeper for this family." }
    },
    sources: ['https://github.com/miskcoo/ugreen_leds_controller/pull/132'],
    note: 'Non-Pro four-bay model with a Celeron N5105. Other specifications remain unconfirmed here. Front and rear case view; no fan control.',
    shots: false,
    fixture: { ram: { type: 'DDR4', mt: 3200 } }
} satisfies Model;
