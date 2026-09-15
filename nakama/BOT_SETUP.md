# Echo VR Discord Bot - Quick Start

## Setup

### 1. Create Discord Bot

1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Click "New Application"
3. Go to "Bot" → "Add Bot"
4. Under "TOKEN" click "Copy" to copy your bot token
5. Paste into `.env` file:
   ```
   DISCORD_BOT_TOKEN=your_token_here
   ```

### 2. Configure Bot Permissions

1. Go to "OAuth2" → "URL Generator"
2. Select scopes: `bot`, `applications.commands`
3. Select permissions:
   - Send Messages
   - Embed Links
   - Read Message History
4. Copy the generated URL and open it to invite bot to your server

### 3. Run the Bot

**Option A: Docker**
```powershell
docker compose -f docker-compose-bot.yml up -d evr-bot
```

**Option B: Local Python**
```powershell
pip install -r requirements.txt
python evr_bot.py
```

## Bot Commands

### `/register-server`
Register a new Echo VR server
```
/register-server server_id: myserver region: us-east server_name: "My Server"
```

### `/list-servers`
List all registered servers
```
/list-servers
```

### `/get-taxi`
Get a taxi link to join a server
```
/get-taxi server_id: myserver player_name: "MyName"
```

### `/post-taxi`
Post all taxi links to current channel
```
/post-taxi
```

### `/help-evr`
Get help on commands
```
/help-evr
```

## Database

Server data is stored in `evr_servers.db` (SQLite)

## Example Usage

1. Register a server:
   ```
   /register-server server_id: echo_server_001 server_name: "Echo's Game" region: us-oklahoma
   ```

2. List servers:
   ```
   /list-servers
   ```

3. Get a taxi link:
   ```
   /get-taxi server_id: echo_server_001 player_name: "YourName"
   ```

4. Post all links to a channel:
   ```
   /post-taxi
   ```

## Support

For issues or features, check the bot logs or contact the maintainer.
