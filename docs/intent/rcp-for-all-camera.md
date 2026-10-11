# Intent: RCP For All Camera

Confirmed with the owner on 2026-10-11.

- **Outcome:** An open-source PC program that makes the ATEM Micro Camera Panel work as a universal camera control panel, with no ATEM switcher. It has modules for the control surface, each camera type and each router/mixer, so new ones can be added later.
- **User:** The owner first (Blackmagic Studio 6K Pro, Sony FX6, Sony FX9). Then anyone else with the same panel who wants to control non-ATEM setups.
- **Why now:** The panel only talks to ATEM gear. With no switcher it is unusable. The owner wants it to control their cameras directly, on the same network.
- **Success:** The panel is connected by USB. Pressing button 1-8 selects the camera and routes that source to the aux output on a Videohub, ATEM or vMix at the same time. The knobs, joystick and buttons then drive every control each camera exposes over Ethernet (iris, gain/ISO, ND, white balance, focus, color correction where the camera allows it, record).
- **Constraint:** No ATEM switcher is available. The program must pose as one or read the panel directly over USB, and it must use each camera's network protocol (Blackmagic ethernet control for the 6K Pro, Sony network control for the FX6 and FX9). The Sony cameras will expose fewer controls than the Blackmagic one.
- **Out of scope:** Being a video switcher, handling video or audio, cameras beyond the three at first (the design allows more later), a polished installer or commercial licensing, cloud or remote access.
