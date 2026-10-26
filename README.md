## ❤️ Support

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

Current release: **V1.4**

### V1.4 — 19 October 2026

- Add another machine by name and URL. The name is a link on the line under this computer.
- Remove a machine from that line. The list stays on this computer.

### V1.3 — 19 October 2026

- The line under the title shows this machine’s name in the accent color.
- The **All sessions** lines fade from the accent color to clear, with the clear end beside the label.
- While a pin is dragged, the rounded card clips the other rows. Only the held row lifts out.

### V1.2 — 12 October 2026

- Pin a session. The copy sits in an accent panel at the top, above an **All sessions** line.
- Drag a pinned row to reorder it. The other rows slide into place.
- Row actions run Move, Pin, Lock, then Delete.

### V1.1 — 11 October 2026

- Collapse or expand each project folder.
- **Collapse all** and **Expand all** sit at the top of the page.

### V1.0 — 11 October 2026

- List every session on the machine, grouped by project folder.
- Delete one session, or several selected sessions.
- Lock a session or a folder so it cannot be moved or deleted until it is unlocked.
- Move a session into another folder that is already listed.
- Copy `grok --resume <session-id>` from a session id, and `cd <path>` from a project path.
- Mark a session that is still open as **Session Active**.

## Run

Python 3.11 or newer. No extra packages.

```bash
python3 server.py
```

The app listens on `127.0.0.1:18090`. Open that address in a browser.

`install.sh` adds an nginx site on port 8090 and a systemd service named `grok-manager`. It needs root.

## What stays on your machine

- Session transcripts, titles, and ids
- `locks.json`, the list of sessions and folders you locked
- `pins.json`, the pinned sessions and their order
- `machines.json`, the other machines you added
- The search index and dashboard database Grok already keeps

Do not commit `locks.json`, `pins.json`, or `machines.json`. They are listed in `.gitignore`.

Deleting a session removes its directory. That cannot be undone.
