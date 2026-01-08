# Plex Media Server API Summary

This document covers the Plex Media Server API endpoints and methods needed for detecting active remote streams.

## Table of Contents

- [Authentication with X-Plex-Token](#authentication-with-x-plex-token)
- [How to Obtain a Plex Token](#how-to-obtain-a-plex-token)
- [The /status/sessions Endpoint](#the-statussessions-endpoint)
- [Detecting Remote vs Local Streams](#detecting-remote-vs-local-streams)
- [Python Code Examples](#python-code-examples)

---

## Authentication with X-Plex-Token

All Plex Media Server API requests require authentication via the `X-Plex-Token` parameter.

### Methods to Include the Token

1. **HTTP Header** (recommended):
   ```
   X-Plex-Token: your_token_here
   ```

2. **Query Parameter**:
   ```
   http://server:32400/status/sessions?X-Plex-Token=your_token_here
   ```

### Required Headers for API Requests

When making requests to Plex APIs (especially plex.tv), include these headers:

| Header | Description |
|--------|-------------|
| `X-Plex-Token` | Your authentication token |
| `X-Plex-Product` | Your application name (e.g., "My Plex App") |
| `X-Plex-Client-Identifier` | Unique identifier for your app instance (UUID) |
| `Accept` | `application/json` for JSON responses (default is XML) |

---

## How to Obtain a Plex Token

### Method 1: Manual Extraction (Temporary Token)

1. Sign in to the Plex Web App
2. Browse to any library item
3. Open browser developer tools (F12)
4. Go to Network tab and filter for requests containing `X-Plex-Token`
5. Copy the token value from any request

**Note:** Tokens obtained this way are temporary and may expire.

### Method 2: PIN-Based Authentication (Programmatic)

This is the recommended method for applications that need persistent authentication.

#### Step 1: Generate a PIN

```bash
curl -X POST 'https://plex.tv/api/v2/pins' \
  -H 'Accept: application/json' \
  -d 'strong=true' \
  -d 'X-Plex-Product=My Plex App' \
  -d 'X-Plex-Client-Identifier=unique-client-id-12345'
```

**Response:**
```json
{
  "id": 123456789,
  "code": "ABCD1234",
  "expiresAt": "2024-01-01T00:30:00Z",
  ...
}
```

#### Step 2: Direct User to Auth URL

Construct the authentication URL:

```
https://app.plex.tv/auth#?clientID=unique-client-id-12345&code=ABCD1234&forwardUrl=YOUR_CALLBACK_URL
```

#### Step 3: Poll for Token (or wait for callback)

```bash
curl -X GET 'https://plex.tv/api/v2/pins/123456789' \
  -H 'Accept: application/json' \
  -d 'code=ABCD1234' \
  -d 'X-Plex-Client-Identifier=unique-client-id-12345'
```

**Response (after user authenticates):**
```json
{
  "id": 123456789,
  "code": "ABCD1234",
  "authToken": "YOUR_PLEX_TOKEN_HERE",
  ...
}
```

#### Step 4: Verify Token is Valid

```bash
curl -X GET 'https://plex.tv/api/v2/user' \
  -H 'Accept: application/json' \
  -H 'X-Plex-Token: YOUR_PLEX_TOKEN_HERE' \
  -d 'X-Plex-Product=My Plex App' \
  -d 'X-Plex-Client-Identifier=unique-client-id-12345'
```

- **200 OK**: Token is valid
- **401 Unauthorized**: Token is invalid or expired

---

## The /status/sessions Endpoint

This endpoint returns information about all currently active playback sessions on the Plex Media Server.

### Endpoint Details

| Property | Value |
|----------|-------|
| **URL** | `GET http://{server_ip}:32400/status/sessions` |
| **Authentication** | Required (`X-Plex-Token`) |
| **Response Format** | XML (default) or JSON (with `Accept: application/json`) |

### Request Examples

**cURL (JSON response):**
```bash
curl -X GET 'http://192.168.1.100:32400/status/sessions' \
  -H 'Accept: application/json' \
  -H 'X-Plex-Token: YOUR_TOKEN_HERE'
```

**cURL (XML response):**
```bash
curl -X GET 'http://192.168.1.100:32400/status/sessions?X-Plex-Token=YOUR_TOKEN_HERE'
```

### Response Status Codes

| Code | Description |
|------|-------------|
| **200** | Success - returns session data |
| **401** | Unauthorized - invalid or missing token |

### JSON Response Structure

```json
{
  "MediaContainer": {
    "size": 1,
    "Metadata": [
      {
        "sessionKey": "123",
        "type": "movie",
        "title": "Movie Title",
        "grandparentTitle": "Show Name",
        "Player": {
          "title": "Living Room TV",
          "address": "192.168.1.50",
          "local": true,
          "machineIdentifier": "abc123def456",
          "model": "Roku Ultra",
          "platform": "Roku",
          "platformVersion": "10.0",
          "product": "Plex for Roku",
          "relayed": false,
          "remotePublicAddress": "",
          "secure": true,
          "state": "playing",
          "userID": 1,
          "vendor": "Roku",
          "version": "5.0.0"
        },
        "Session": {
          "id": "session-id-here",
          "bandwidth": 20000,
          "location": "lan"
        },
        "User": {
          "id": "1",
          "title": "username",
          "thumb": "https://plex.tv/users/abc/avatar"
        },
        "TranscodeSession": {
          "key": "/transcode/sessions/abc123",
          "throttled": false,
          "complete": false,
          "progress": 5.0,
          "speed": 2.5,
          "videoDecision": "transcode",
          "audioDecision": "copy"
        }
      }
    ]
  }
}
```

### XML Response Structure

```xml
<?xml version="1.0" encoding="UTF-8"?>
<MediaContainer size="1">
  <Video sessionKey="123" type="movie" title="Movie Title">
    <Player
      title="Living Room TV"
      address="192.168.1.50"
      local="1"
      machineIdentifier="abc123def456"
      platform="Roku"
      product="Plex for Roku"
      state="playing"
      remotePublicAddress=""
      relayed="0"
      secure="1"
    />
    <Session id="session-id" bandwidth="20000" location="lan" />
    <User id="1" title="username" thumb="https://plex.tv/users/abc/avatar" />
    <TranscodeSession key="/transcode/sessions/abc123" videoDecision="transcode" />
  </Video>
</MediaContainer>
```

### Empty Response (No Active Sessions)

```json
{
  "MediaContainer": {
    "size": 0
  }
}
```

---

## Detecting Remote vs Local Streams

### Key Fields for Detection

The following fields from the session response are essential for determining if a stream is local or remote:

#### Player Object Fields

| Field | Type | Description |
|-------|------|-------------|
| `Player.local` | Boolean | **Primary indicator** - `true` if client is on the local LAN, `false` if remote |
| `Player.address` | String | The client's IP address (local/private IP) |
| `Player.remotePublicAddress` | String | The client's public IP address (populated for remote connections) |
| `Player.relayed` | Boolean | `true` if the connection is relayed through Plex's relay servers |
| `Player.secure` | Boolean | `true` if the connection is encrypted/secure |
| `Player.state` | String | Playback state: `"playing"`, `"paused"`, `"buffering"` |

#### Session Object Fields

| Field | Type | Description |
|-------|------|-------------|
| `Session.location` | String | `"lan"` for local network, `"wan"` for remote |
| `Session.bandwidth` | Integer | Current bandwidth usage in kbps |

### Detection Logic

```python
def is_remote_stream(session):
    """
    Determine if a session is a remote stream.

    Returns True if the stream is remote (not on local LAN).
    """
    player = session.get('Player', {})
    session_info = session.get('Session', {})

    # Primary check: Player.local field
    is_local = player.get('local', True)

    # Secondary check: Session.location field
    location = session_info.get('location', 'lan')

    # Tertiary check: remotePublicAddress is populated for remote streams
    has_remote_address = bool(player.get('remotePublicAddress', ''))

    # A stream is remote if:
    # 1. Player.local is False, OR
    # 2. Session.location is not 'lan', OR
    # 3. remotePublicAddress is populated
    return not is_local or location != 'lan' or has_remote_address
```

### How Plex Determines Local vs Remote

Plex uses the IP address space of the primary network interface to establish the local network boundary:

- **Local**: Clients connecting from the same subnet as the server
- **Remote**: Clients connecting from external/public IP addresses
- Connections through reverse proxies are typically seen as remote
- VPN connections (e.g., Tailscale) are considered remote
- Accessing via public IP (even from local network) is considered remote

---

## Python Code Examples

### Using the python-plexapi Library

Install the library:
```bash
pip install plexapi
```

### Basic Connection

```python
from plexapi.server import PlexServer

# Connect to your Plex server
PLEX_URL = 'http://192.168.1.100:32400'
PLEX_TOKEN = 'your_plex_token_here'

plex = PlexServer(PLEX_URL, PLEX_TOKEN)
```

### Get All Active Sessions

```python
from plexapi.server import PlexServer

def get_active_sessions(plex_url, plex_token):
    """Get all active playback sessions."""
    plex = PlexServer(plex_url, plex_token)
    sessions = plex.sessions()

    for session in sessions:
        print(f"Title: {session.title}")
        print(f"Type: {session.type}")
        print(f"User: {session.usernames[0] if session.usernames else 'Unknown'}")

        if session.players:
            player = session.players[0]
            print(f"Player: {player.title}")
            print(f"Platform: {player.platform}")
            print(f"State: {player.state}")
        print("---")

    return sessions
```

### Detect Remote Streams

```python
from plexapi.server import PlexServer

def get_remote_sessions(plex_url, plex_token):
    """
    Get all remote (non-local) streaming sessions.

    Returns a list of session objects that are streaming remotely.
    """
    plex = PlexServer(plex_url, plex_token)
    sessions = plex.sessions()

    remote_sessions = []

    for session in sessions:
        if not session.players:
            continue

        player = session.players[0]

        # Check if the stream is remote
        is_local = getattr(player, 'local', True)
        is_relayed = getattr(player, 'relayed', False)
        remote_address = getattr(player, 'remotePublicAddress', '')

        if not is_local or is_relayed or remote_address:
            remote_sessions.append({
                'title': session.title,
                'type': session.type,
                'user': session.usernames[0] if session.usernames else 'Unknown',
                'player_name': player.title,
                'player_platform': player.platform,
                'state': player.state,
                'is_local': is_local,
                'is_relayed': is_relayed,
                'remote_address': remote_address,
                'player_address': getattr(player, 'address', ''),
            })

    return remote_sessions


# Example usage
if __name__ == '__main__':
    PLEX_URL = 'http://192.168.1.100:32400'
    PLEX_TOKEN = 'your_token_here'

    remote = get_remote_sessions(PLEX_URL, PLEX_TOKEN)

    if remote:
        print(f"Found {len(remote)} remote stream(s):")
        for session in remote:
            print(f"  - {session['user']} watching '{session['title']}' "
                  f"on {session['player_name']} ({session['player_platform']})")
            print(f"    Remote IP: {session['remote_address']}")
    else:
        print("No remote streams active.")
```

### Using Raw HTTP Requests (without plexapi library)

```python
import requests

def get_sessions_raw(plex_url, plex_token):
    """
    Get active sessions using raw HTTP requests.
    Returns JSON response.
    """
    url = f"{plex_url}/status/sessions"
    headers = {
        'Accept': 'application/json',
        'X-Plex-Token': plex_token
    }

    response = requests.get(url, headers=headers)
    response.raise_for_status()

    return response.json()


def detect_remote_streams_raw(plex_url, plex_token):
    """
    Detect remote streams using raw API calls.
    """
    data = get_sessions_raw(plex_url, plex_token)

    media_container = data.get('MediaContainer', {})
    sessions = media_container.get('Metadata', [])

    remote_streams = []

    for session in sessions:
        player = session.get('Player', {})
        session_info = session.get('Session', {})
        user = session.get('User', {})

        is_local = player.get('local', True)
        location = session_info.get('location', 'lan')
        remote_address = player.get('remotePublicAddress', '')

        # Determine if remote
        is_remote = not is_local or location != 'lan' or bool(remote_address)

        if is_remote:
            remote_streams.append({
                'title': session.get('title', 'Unknown'),
                'type': session.get('type', 'unknown'),
                'user': user.get('title', 'Unknown'),
                'player_name': player.get('title', 'Unknown'),
                'player_platform': player.get('platform', 'Unknown'),
                'state': player.get('state', 'unknown'),
                'remote_address': remote_address,
                'local_address': player.get('address', ''),
                'is_relayed': player.get('relayed', False),
                'bandwidth': session_info.get('bandwidth', 0),
            })

    return remote_streams


# Example usage
if __name__ == '__main__':
    PLEX_URL = 'http://192.168.1.100:32400'
    PLEX_TOKEN = 'your_token_here'

    try:
        remote = detect_remote_streams_raw(PLEX_URL, PLEX_TOKEN)
        print(f"Remote streams: {len(remote)}")
        for stream in remote:
            print(f"  {stream['user']}: {stream['title']} "
                  f"(from {stream['remote_address'] or stream['local_address']})")
    except requests.exceptions.RequestException as e:
        print(f"Error connecting to Plex: {e}")
```

### PIN Authentication Flow in Python

```python
import requests
import time
import uuid

class PlexAuth:
    """Handle Plex PIN-based authentication."""

    def __init__(self, app_name, client_id=None):
        self.app_name = app_name
        self.client_id = client_id or str(uuid.uuid4())
        self.base_url = 'https://plex.tv/api/v2'

    def get_pin(self):
        """Request a new PIN from Plex."""
        url = f"{self.base_url}/pins"
        data = {
            'strong': 'true',
            'X-Plex-Product': self.app_name,
            'X-Plex-Client-Identifier': self.client_id
        }
        headers = {'Accept': 'application/json'}

        response = requests.post(url, data=data, headers=headers)
        response.raise_for_status()

        result = response.json()
        return {
            'id': result['id'],
            'code': result['code'],
            'auth_url': f"https://app.plex.tv/auth#?clientID={self.client_id}&code={result['code']}"
        }

    def check_pin(self, pin_id, pin_code):
        """Check if the PIN has been claimed and get the token."""
        url = f"{self.base_url}/pins/{pin_id}"
        params = {
            'code': pin_code,
            'X-Plex-Client-Identifier': self.client_id
        }
        headers = {'Accept': 'application/json'}

        response = requests.get(url, params=params, headers=headers)
        response.raise_for_status()

        result = response.json()
        return result.get('authToken')

    def wait_for_auth(self, pin_id, pin_code, timeout=300, poll_interval=1):
        """Poll for authentication with timeout."""
        start_time = time.time()

        while time.time() - start_time < timeout:
            token = self.check_pin(pin_id, pin_code)
            if token:
                return token
            time.sleep(poll_interval)

        raise TimeoutError("Authentication timed out")


# Example usage
if __name__ == '__main__':
    auth = PlexAuth('My Plex Helper App')

    # Get PIN
    pin_info = auth.get_pin()
    print(f"Please visit: {pin_info['auth_url']}")
    print(f"Or enter PIN: {pin_info['code']} at https://plex.tv/link")

    # Wait for user to authenticate
    try:
        token = auth.wait_for_auth(pin_info['id'], pin_info['code'])
        print(f"Authentication successful!")
        print(f"Your token: {token}")
    except TimeoutError:
        print("Authentication timed out. Please try again.")
```

---

## References

- [Plex Support: Finding an authentication token / X-Plex-Token](https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/)
- [Plex Developer Documentation](https://developer.plex.tv/pms/)
- [Plex API Documentation (plexapi.dev)](https://plexapi.dev/api-reference/sessions/get-active-sessions)
- [Python PlexAPI Documentation](https://python-plexapi.readthedocs.io/en/latest/)
- [Plex Forum: Authenticating with Plex](https://forums.plex.tv/t/authenticating-with-plex/609370)
- [Plexopedia: Get Active Sessions](https://www.plexopedia.com/plex-media-server/api/server/sessions/)
- [GitHub: Plex Web API Overview](https://github.com/Arcanemagus/plex-api/wiki/Plex-Web-API-Overview)
