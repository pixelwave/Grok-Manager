# Grok Build - Sessions Manager


<img width="2298" height="662" alt="Mask group-1" src="https://github.com/user-attachments/assets/0cfb4c59-df5d-4d83-8798-bf46b3329fd5" />


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

<img width="2298" height="1228" alt="Mask group" src="https://github.com/user-attachments/assets/839300ae-1793-41b9-bf78-79187f5c92a5" />

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

Deleting a session removes its directory. That cannot be undone.
