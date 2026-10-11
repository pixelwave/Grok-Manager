# Grok Manager

A local page for the Grok sessions on one machine. Sessions are grouped by project folder. You can lock a session or a folder, move a session to another folder, copy a resume command, and delete sessions you no longer need.

It reads `~/.grok/sessions` on the computer where it runs. Nothing is uploaded.

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
- The search index and dashboard database Grok already keeps

Do not commit `locks.json`. It is listed in `.gitignore`.

## Actions

- Copy a session id to get `grok --resume <session-id>`.
- Copy a project path to get `cd <path>`.
- Move sends the session directory to another folder that is already listed, and updates the stored working directory. The transcript is not rewritten.
- A session that is open, or locked, cannot be moved or deleted until that changes.
- Locking a folder blocks delete and move for every session in that folder.

Deleting a session removes its directory. That cannot be undone.
