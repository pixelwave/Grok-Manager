If you find my work useful, please consider supporting:

[![GitHub Sponsors](https://img.shields.io/badge/Sponsor-pixelWAVE-ea4aaa?logo=github-sponsors&logoColor=white)](https://donate.p-wave.de/)


# Grok Build - Sessions Manager

<img width="2130" height="1560" alt="Group 85" src="https://github.com/user-attachments/assets/eef9f790-950e-4fad-a865-4e034784f44b" />

A local page for the Grok sessions on one machine, grouped by project folder. You can pin, lock, move, and delete them. The page reads `~/.grok/sessions` where it runs. Nothing is uploaded.

## Highlights

- Copy a session id as `grok --resume <session-id>`, or a project path as `cd <path>`.
- Move a session into another listed folder. The transcript stays as it was written.
- Lock a session or a folder so it cannot be moved or deleted until you unlock it. An open session is marked **Session Active** and stays put.
- Collapse one project, or use **Collapse all** at the top.
- Pin a session to the accent panel and drag it to reorder.
- The line under the title names this computer. **ADD** saves another machine by name and URL. Managers on the same network show up there on their own.
- **Update available**, under that line, installs a newer release on this machine.

<img width="2298" height="1228" alt="Mask group" src="https://github.com/user-attachments/assets/839300ae-1793-41b9-bf78-79187f5c92a5" />

## Releases

Current version: **26.10.29.7**. A release number is the day it is published, `YY.MM.DD`. Notes are in [CHANGELOG.md](CHANGELOG.md).

## Install

Linux and Mac:

```bash
curl -fsSL https://pixelwave.github.io/grok-manager/install.sh | bash
```

Windows PowerShell:

```powershell
irm https://pixelwave.github.io/grok-manager/install.ps1 | iex
```

The installer picks the binary for this machine. The page listens on port 18090. To choose another port:

```bash
curl -fsSL https://pixelwave.github.io/grok-manager/install.sh | bash -s -- -p 6969
```

```powershell
& ([scriptblock]::Create((irm https://pixelwave.github.io/grok-manager/install.ps1))) -p 6969
```

Root installs for root’s sessions, at `/usr/local/bin`. Any other account installs for that account’s sessions, at `~/.local/bin`.

```bash
grok-manager up
grok-manager down
grok-manager status
grok-manager update
```

`status` prints the version, host name, and URL. `update` installs the latest release.

<img width="515" height="310" alt="Screenshot 2026-10-29 at 08 22 10" src="https://github.com/user-attachments/assets/c94896cc-f7af-4fea-bf94-765efa500771" />

Uninstall, and add `--purge` to also delete saved locks, pins, and machines:

```bash
curl -fsSL https://pixelwave.github.io/grok-manager/uninstall.sh | bash
```

```powershell
irm https://pixelwave.github.io/grok-manager/uninstall.ps1 | iex
```

## From a checkout

Go 1.24 or newer. This builds and restarts the local service. It does not publish a release.

```bash
./scripts/local-install.sh
```

## On this machine

Locks, pins, and saved machines stay in the config directory. Do not commit `locks.json`, `pins.json`, or `machines.json`. Deleting a session removes its directory.

## License

Grok Manager is free software under the GNU Affero General Public License, version 3 only. Copyright (C) 2026 Denny Moritz. See `LICENSE` and `COPYRIGHT`.

You can download it and run it for yourself. If you modify Grok Manager and let other people use that modified version over a network, the AGPL requires you to offer them the corresponding source.

Session transcripts, locks, pins, and saved machines are your data. The license applies to the program, not to that data.

The corresponding source for an official release is the git tag that built it.
