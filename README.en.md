# multi-kimi

English | [简体中文](README.md)

multi-kimi — a desktop tool for switching login credentials across multiple **Kimi Code** accounts with one click, plus a live view of each account's usage.

## Features

- **Multi-account switching**: save as many accounts as you like (e.g. `work` / `personal`) and switch instantly — no re-login needed;
- **Usage overview**: each account card shows 5-hour usage, weekly usage, and reset countdowns, refreshed on demand;
- **Tray-friendly**: closing the window keeps the app in the system tray; double-click the icon to bring it back;
- **Safe switching**: credential updates made by the CLI are synced automatically during a switch, so stale tokens never overwrite fresh ones;
- **Local only**: credentials are copied between two local directories and never touch any third party; usage queries go only to the official Kimi API;
- **Privacy first**: credentials are never displayed in the UI or logs; storage uses private local permissions.

## Download

Get the desktop app for your platform from [Releases](../../releases):

| Platform | File |
| --- | --- |
| Windows x64 | `multi-kimi-*-windows-amd64.exe` |
| Windows ARM64 | `multi-kimi-*-windows-arm64.exe` |
| macOS Intel | `multi-kimi-*-darwin-amd64.app.zip` |
| macOS Apple Silicon | `multi-kimi-*-darwin-arm64.app.zip` |

On macOS, if Gatekeeper blocks the first launch, right-click the app in Finder and choose **Open**, or allow it under **System Settings → Privacy & Security**.

## Usage

1. Sign in to your first account in a terminal with `kimi login`;
2. Open multi-kimi, click **Save**, and name the account (e.g. `work`);
3. Sign out in the CLI, sign in with another account, and save again (e.g. `personal`);
4. From then on, click **Switch** any time to move between accounts without logging in again.

Other actions:

- **Recapture**: refresh an account's snapshot from the current login (the tool prompts when the live credentials belong to a different account);
- **Refresh usage**: click the refresh button on a card header to update usage for all accounts;
- **Delete**: removes the local snapshot only — your current login is untouched;
- **Storage folder**: opens the local storage directory in Explorer / Finder.

## Data & Security

- Account snapshots live in `~/.multi-kimi/`; your active login credentials stay in the CLI's own directory — neither interferes with the other;
- All data stays on your machine except requests to the official Kimi API — no third-party services involved;
- If environment variables that could bypass the credentials file are detected (e.g. `KIMI_CODE_HOME`), you get a warning before switching (variable names only).
