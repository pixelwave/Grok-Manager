If you find my work useful, please consider supporting:

[![GitHub Sponsors](https://img.shields.io/badge/Sponsor-pixelWAVE-ea4aaa?logo=github-sponsors&logoColor=white)](https://donate.p-wave.de/)


# Grok Build - Sessions Manager


<img width="2298" height="1594" alt="Mask group" src="https://github.com/user-attachments/assets/1c49ce81-4d38-45c5-8613-a7b8a55af25f" />


A local page for the Grok sessions on one machine. Sessions are grouped by project folder. You can lock a session or a folder, move a session, and delete one you no longer need.

The page reads `~/.grok/sessions` on the computer where it runs. Nothing is uploaded.

<img width="2298" height="1632" alt="grok-manager" src="https://github.com/user-attachments/assets/b9b2e89f-7291-4b10-bb21-4349e8c5c084" />

## Highlights

- Click a session id and the clipboard gets `grok --resume <session-id>`, ready to paste into a terminal.
- Click a project path, such as `/root/projects/plannr`, and the clipboard gets `cd <path>`.
- The folder icon moves a session into another folder that is already listed. The transcript is left as it was written.
- The lock icon keeps one session, or every session in a folder, from being moved or deleted until you unlock it.
- A session that is still open is marked **Session Active**. It cannot be moved or deleted while it is open.
- Click a project name to collapse or expand the sessions in that folder.
- **Collapse all**, at the top of the page, folds every visible folder. The same control expands them again.
- The pin icon copies a session into an accent panel at the top. The original stays in its folder.
- Drag the handle on a pinned row to reorder it. The new order is kept on this machine.
- The line under the title names this computer, in the accent color.
- **ADD** saves another machine by name and URL. The name links to that manager, and you can remove it again.

<img width="2298" height="1228" alt="Mask group" src="https://github.com/user-attachments/assets/839300ae-1793-41b9-bf78-79187f5c92a5" />

## Releases

Current version: **26.10.29.7**. Notes for each version are in [CHANGELOG.md](CHANGELOG.md).

## Install

Linux and Mac:

```bash
curl -fsSL https://pixelwave.github.io/grok-manager/install.sh | bash
```

Windows PowerShell:

```powershell
irm https://pixelwave.github.io/grok-manager/install.ps1 | iex
```

The installer picks the binary for this machine. An unsupported system stops with a short message. The page listens on port 18090. Choose another port with `-p`:

```bash
curl -fsSL https://pixelwave.github.io/grok-manager/install.sh | bash -s -- -p 6969
```

On Windows, pass the port as:

```powershell
& ([scriptblock]::Create((irm https://pixelwave.github.io/grok-manager/install.ps1))) -p 6969
```

A normal account installs a user service and reads that account’s Grok sessions. Root does the same for root’s sessions. The command is placed on `/usr/local/bin` for root and on `~/.local/bin` for anyone else. Add `~/.local/bin` to `PATH` when the shell does not already search it.

```bash
grok-manager up
grok-manager down
grok-manager status
grok-manager update
```

`status` prints the version, host name, and URL. `update` installs the latest release and removes the previous binary. The page shows **Update available** when a newer release exists. One click updates that machine.

Remove it again with the matching uninstaller. Add `--purge` to delete the saved locks, pins, and machines too.

```bash
curl -fsSL https://pixelwave.github.io/grok-manager/uninstall.sh | bash
```

```powershell
irm https://pixelwave.github.io/grok-manager/uninstall.ps1 | iex
```

GitHub Pages for this repo should serve the `docs/` folder so those short URLs resolve. The pages scripts download the installers attached to the latest release.

## Run from a checkout

Build the checkout and restart the service on this machine:

```bash
./scripts/local-install.sh
```

That does not publish a release. `grok-manager update` installs a GitHub release only when that release is newer than the running build, so this local build stays in place. Tag a release when the change is ready for other machines. Go 1.24 or newer is required. The page listens on port 18090.

## What stays on your machine

- Session transcripts, titles, and ids
- `locks.json`, the list of sessions and folders you locked
- `pins.json`, the pinned sessions and their order
- `machines.json`, the other machines you added
- The search index and dashboard database Grok already keeps

Do not commit `locks.json`, `pins.json`, or `machines.json`. They are listed in `.gitignore`.

Deleting a session removes its directory. That cannot be undone.
