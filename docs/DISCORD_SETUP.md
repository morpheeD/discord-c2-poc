# Setup Guide: Creating a Discord Bot

To use this C2 PoC, you need to create a Discord Bot and invite it to your server. Follow these steps:

## 1. Create the Application
1.  Go to the [Discord Developer Portal](https://discord.com/developers/applications).
2.  Log in with your Discord account.
3.  Click the **"New Application"** button (top right).
4.  Give it a name (e.g., "C2-Agent") and click **Create**.

## 2. Create the Bot User
1.  In the left menu, click **"Bot"**.
2.  Click **"Reset Token"** to generate your **Token**.
    *   **IMPORTANT:** Copy this token immediately. You will not be able to see it again.
    *   Paste this token into your `.env` file as `DISCORD_TOKEN`.
3.  Scroll down to the **"Privileged Gateway Intents"** section.
4.  **Enable** the following intents:
    *   **Message Content Intent** (Required to read commands like `!exec`).
    *   **Server Members Intent** (Optional, but good for stability).
5.  Click **"Save Changes"**.

## 3. Invite the Bot to Your Server
1.  In the left menu, click **"OAuth2"** -> **"URL Generator"**.
2.  Under **"Scopes"**, check the box for **`bot`**.
3.  Under **"Bot Permissions"**, check:
    *   `Read Messages/View Channels`
    *   `Send Messages`
    *   `Read Message History`
4.  Copy the **Generated URL** at the bottom.
5.  Paste the URL into your browser, select your private server, and click **Authorize**.

## 4. Get Channel IDs
1.  Open your Discord User Settings (gear icon).
2.  Go to **Advanced** and enable **Developer Mode**.
3.  Go to your server and create two channels: `#commands` and `#results`.
4.  Right-click on `#commands` and select **"Copy Channel ID"**.
    *   Paste this into `.env` as `COMMAND_CHANNEL_ID`.
5.  Right-click on `#results` and select **"Copy Channel ID"**.
    *   Paste this into `.env` as `RESULT_CHANNEL_ID`.

## 5. Generate Encryption Key
You need a 32-byte key for AES-256. You can generate one using Python or an online tool (be careful with online tools for real secrets).

**Python one-liner:**
```python
import secrets; print(secrets.token_hex(16))
```
*   Paste the result into `.env` as `ENCRYPTION_KEY`.
