import family from './idx6011';
import type { Model } from './schema';

export default {
    ...family,
    id: 'idx6011-plain',
    name: 'iDX6011',
    dmiPattern: '^iDX6011$',
    fixtureDmi: 'iDX6011',
    cpu: 'Intel Core Ultra 5 125H',
    fan: null,
    verified: {
        ...family.verified,
        leds: {
            status: 'community',
            by: 'deiviest (issue 318)',
            version: '5.6.0',
            date: '2026-11-14',
            ref: 'https://github.com/ugreen-nas-control/ugreen-nas-control/issues/318',
            note: 'Confirmed on the plain iDX6011 with 5.6.0 and Unraid 7.3.4: MCU at 0x3a on I801, Identify lights LAN and disks 1-6 in front-label order, power colour and brightness changes work. Status mode with activity disabled keeps sleeping drives asleep, dims occupied bays and turns empty bays off. Physical SATA-to-bay mapping checked only for occupied bays 1 and 2; activity remains untested. EC coexistence with ug-fand is tracked separately in #318.'
        },
        fans: {
            status: 'unsupported',
            note: 'Fan control is not offered on the plain model. The backend can read its two system fans (mode 0x9c/0x9e, duty 0x9d/0x9f, tach 0x96-0x99), but never writes fan registers. Shared EC access with ug-fand is tracked in #318.'
        }
    },
    sources: [
        ...family.sources,
        'https://github.com/ugreen-nas-control/ugreen-nas-control/issues/318',
        'https://github.com/miskcoo/ugreen_leds_controller/issues/93'
    ],
    note: 'Plain six-bay iDX6011: LED control confirmed with 5.6.0. No fan control; read-only fan backend. Activity and full physical bay mapping remain unconfirmed.',
    shots: false,
    fixture: { cpu: 'Intel Core Ultra 5 125H' }
} satisfies Model;
