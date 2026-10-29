# Changelog

A release number is the day it is published, `YY.MM.DD`. 28 October 2026 is `26.10.28`. Another release that same day is `26.10.28.2`, then `26.10.28.3`. The next day starts again at three numbers, `26.10.29`.

## 26.10.29.6 — 29 October 2026

- The page finds this account’s sessions when the service has no home directory set.

## 26.10.29.5 — 29 October 2026

- Installing over a service that is already running restarts it, so the new process is the one that listens.

## 26.10.29.4 — 29 October 2026

- `grok-manager status` shows the address and port on one line, `URL`.

## 26.10.29.3 — 29 October 2026

- After install, the command prints `For more info use "grok-manager status"`.

## 26.10.29.2 — 29 October 2026

- `grok-manager status` prints the GROK banner with the version, host name, address, and port.
- Release notes live in this file. The README links here.

## 26.10.29 — 29 October 2026

- Release numbers follow the publish date.
- Install from one line on Linux, Mac, and Windows. `grok-manager up`, `down`, `status`, and `update` control that install.
- Managers on the same network appear as links. A saved machine stays in the list.

## V1.4 — 19 October 2026

- Add another machine by name and URL. The name is a link on the line under this computer.
- Remove a machine from that line. The list stays on this computer.

## V1.3 — 19 October 2026

- The line under the title shows this machine’s name in the accent color.
- The **All sessions** lines fade from the accent color to clear, with the clear end beside the label.
- While a pin is dragged, the rounded card clips the other rows. Only the held row lifts out.

## V1.2 — 12 October 2026

- Pin a session. The copy sits in an accent panel at the top, above an **All sessions** line.
- Drag a pinned row to reorder it. The other rows slide into place.
- Row actions run Move, Pin, Lock, then Delete.

## V1.1 — 11 October 2026

- Collapse or expand each project folder.
- **Collapse all** and **Expand all** sit at the top of the page.

## V1.0 — 11 October 2026

- List every session on the machine, grouped by project folder.
- Delete one session, or several selected sessions.
- Lock a session or a folder so it cannot be moved or deleted until it is unlocked.
- Move a session into another folder that is already listed.
- Copy `grok --resume <session-id>` from a session id, and `cd <path>` from a project path.
- Mark a session that is still open as **Session Active**.
