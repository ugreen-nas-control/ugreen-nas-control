/** People credited in the README (rendered by gen-docs.ts). Add the reporter's GitHub login when a model report is recorded (docs/hardware.md). */
export const TESTERS: { login: string; did: string }[] = [
    { login: 'manilx', did: 'DXP6800 Pro confirmation on 5.6.0 (#302), fan-curve feedback (#304), history time-zone report (#305)' },
    { login: 'snipah', did: 'CPU spike measurements, diagnostic serial-scrubbing report and LAN investigation (#306)' },
    { login: 'Waxonwaxoffmyson', did: 'DXP4800 Plus diagnostic and model report (#317)' },
    { login: 'sbbbsbbb', did: 'Non-Pro DX4600: upstream CLI LED control test on DSM (miskcoo PR 132)' },
    { login: 'deiviest', did: 'Plain iDX6011: LED control, Identify, standby status and EC coexistence report (#318)' },
    { login: '321chills', did: 'DXP2800 GT: wiring research and fan control test' },
    { login: 'miskcoo', did: 'LED protocol and bay mapping (ugreen_leds_controller driver)' },
    { login: 'datarepos76', did: 'DXP480T Plus: power LED reports (#145), DXP480T Plus report with diag (#271)' },
    { login: 'Chiconydu69', did: 'DXP8800 Plus: fan wiring, header 4 confirmation, NVMe root ports, front-panel button tests, beeper test' },
    { login: 'ivolution', did: 'Spanish translation, DXP4800 Plus tests, RAM layout and NVMe slot mapping/photos (#297), UI bug reports' },
    { login: 'DanielRuppen', did: 'DXP2800 SUB2: ACPI and PCI data, lpss-probe runs, LED path confirmation' },
    { login: 'jsaltzman', did: 'DXP6800 Pro bug reports, the cause of the failing update check' },
    { login: 'halloleo', did: 'DXP8800 Ultra: diagnostics, LED read/write tests and probe-message report (#293)' },
    { login: 'rogvalinger-ops', did: 'DXP4800 testing, dual-LAN and single-RAM-slot corrections with photos (#299)' }
];

/** Testers on the Unraid forum without a known GitHub account: named, not linked. */
export const FORUM_TESTERS = ['fkiesler', 'gigios', 'NullNull67', 'cardsdoc', 'gonzlobo', 'NotRetarded'];

/** Maintainers, shown first in the README credits. */
export const MAINTAINERS: { login: string; name: string }[] = [
    { login: 'j3n57h0m45', name: 'Jens Thomas' },
    { login: 'alexanderschubert', name: 'Alexander Schubert' }
];
