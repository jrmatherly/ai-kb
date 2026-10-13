# Smart Home Hub Pro Product Manual

**Model**: HUB-Pro-2024\
**Document version**: v2.1\
**Firmware**: ≥ 3.4.0

## 1. Product overview

Smart Home Hub Pro is NovaTech's home IoT hub. It manages lights, air conditioners, curtains, security sensors and other devices from one place. Users set up scene automations from the mobile app, a voice assistant or the local touchscreen.

Key selling points:

- Works offline: configured scenes still run on the LAN when the internet is down
- Multi-protocol: supports Matter, Zigbee 3.0, Wi-Fi and Bluetooth Mesh at the same time
- Edge AI: a built-in lightweight model understands casual commands such as "I'm home" or "movie time"

## 2. Specifications

| Item | Value |
| --- | --- |
| Processor | Quad-core ARM Cortex-A55, 1.8 GHz |
| Memory / storage | 4 GB RAM / 32 GB eMMC |
| Wireless | Wi-Fi 6, Zigbee 3.0, Bluetooth 5.2, Thread |
| Wired ports | Gigabit Ethernet ×1, USB-C (debug) ×1 |
| Power | DC 12V / 2A, typical draw 8W |
| Operating temperature | 0℃ ~ 40℃ |
| Max connected devices | 256 (≤ 120 recommended for fast response) |

## 3. First-time setup

1. Plug in the device. A slow blue blink means it is waiting for network setup.
2. Open the NovaHome app, choose "Add hub" and scan the QR code on the device.
3. Follow the wizard to join your home Wi-Fi. On a wired network you can skip the Wi-Fi step.
4. Run the firmware check. If an update is available, upgrade before adding sub-devices.

> **Note**: During first-time setup the phone must be on the same 2.4 GHz band as the hub. 5 GHz-only routers must enable 2.4 GHz compatibility mode first.

## 4. Scene automation examples

### 4.1 "Home mode"

Triggers (any one is enough):

- The phone's GPS enters the home geofence
- The front door fingerprint lock unlocks
- Someone says "I'm back"

Actions:

- Set the living room main light to 70% warm white
- Set the air conditioner to cool at 26℃ (summer template only)
- Disarm security

### 4.2 "Away mode"

Trigger: every household member's phone has been outside the geofence for more than 5 minutes.

Actions: turn off all lights, turn off the air conditioner, arm security, close the gas valve actuator (if connected).

## 5. FAQ

**Q: Does voice control still work when the hub is offline?**\
A: If the voice module uses cloud recognition, only the app and the local touchscreen work without internet. With a local voice pack configured, basic commands keep working.

**Q: How many hubs can one account bind?**\
A: Up to 3 on the Personal edition. The Enterprise edition is licensed per contract, 50 by default.

**Q: What is the warranty policy?**\
A: The unit has a 24-month warranty; battery accessories have 12 months. Damage from disassembly or water is not covered.

## 6. Related documents

- Installation and wiring: *Hub Pro Hardware Installation Guide*
- Expense rules and taking devices on business trips: *Employee Handbook: Expenses and Assets*
- Q1 feature roadmap: *Product Meeting Notes: Q1 Planning*
