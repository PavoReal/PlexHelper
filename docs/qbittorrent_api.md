# qBittorrent Web API Documentation

This document covers the qBittorrent Web API endpoints needed for programmatically managing upload speed limits.

**Official Documentation:**
- [WebUI API (qBittorrent 4.1-4.6.x)](https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1))
- [WebUI API (qBittorrent 5.0+)](https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-5.0))

---

## Authentication Flow

qBittorrent uses **cookie-based authentication**. All API methods require authentication except the login endpoint.

### 1. Login

**Endpoint:** `POST /api/v2/auth/login`

**Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `username` | string | WebUI username |
| `password` | string | WebUI password |

**Headers Required:**
- `Referer` or `Origin`: Must match the exact domain and port of the request URL (for CSRF protection)

**Response:**
- **200 OK**: Login successful. Response includes `Set-Cookie: SID=<session_id>; path=/`
- **403 Forbidden**: IP banned due to too many failed login attempts

**Example Request:**
```bash
curl -i \
  --header 'Referer: http://localhost:8080' \
  --data 'username=admin&password=adminadmin' \
  http://localhost:8080/api/v2/auth/login
```

**Example Response Headers:**
```
HTTP/1.1 200 OK
Set-Cookie: SID=abcdef123456; path=/
Content-Type: text/plain
```

### 2. Using the Session Cookie

For all subsequent requests, include the SID cookie:
```
Cookie: SID=abcdef123456
```

### 3. Logout

**Endpoint:** `POST /api/v2/auth/logout`

**Parameters:** None

**Response:** Always returns HTTP 200

---

## Key Endpoints

### Get Global Transfer Info

Returns global transfer statistics displayed in qBittorrent's status bar.

**Endpoint:** `GET /api/v2/transfer/info`

**Parameters:** None

**Response (JSON):**
```json
{
  "dl_info_speed": 1234567,
  "dl_info_data": 12345678901,
  "up_info_speed": 234567,
  "up_info_data": 1234567890,
  "dl_rate_limit": 0,
  "up_rate_limit": 1048576,
  "dht_nodes": 150,
  "connection_status": "connected"
}
```

**Response Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `dl_info_speed` | int | Current download rate (bytes/s) |
| `dl_info_data` | int | Total downloaded this session (bytes) |
| `up_info_speed` | int | Current upload rate (bytes/s) |
| `up_info_data` | int | Total uploaded this session (bytes) |
| `dl_rate_limit` | int | Current download limit (bytes/s, 0 = unlimited) |
| `up_rate_limit` | int | Current upload limit (bytes/s, 0 = unlimited) |
| `dht_nodes` | int | Number of connected DHT nodes |
| `connection_status` | string | One of: `connected`, `firewalled`, `disconnected` |

---

### Get Global Upload Limit

**Endpoint:** `GET /api/v2/transfer/uploadLimit`

**Parameters:** None

**Response:** Plain text integer - current global upload limit in bytes/second. Returns `0` if unlimited.

**Example:**
```bash
curl --cookie "SID=abcdef123456" \
  http://localhost:8080/api/v2/transfer/uploadLimit
```

**Response:**
```
1048576
```

---

### Set Global Upload Limit

**Endpoint:** `POST /api/v2/transfer/setUploadLimit`

**Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | int | Upload speed limit in bytes/second. Use `0` for unlimited. |

**Response:** HTTP 200 (no body)

**Example - Set 1 MB/s limit:**
```bash
curl -X POST \
  --cookie "SID=abcdef123456" \
  --data "limit=1048576" \
  http://localhost:8080/api/v2/transfer/setUploadLimit
```

**Example - Remove limit (unlimited):**
```bash
curl -X POST \
  --cookie "SID=abcdef123456" \
  --data "limit=0" \
  http://localhost:8080/api/v2/transfer/setUploadLimit
```

---

### Get Global Download Limit

**Endpoint:** `GET /api/v2/transfer/downloadLimit`

**Parameters:** None

**Response:** Plain text integer - current global download limit in bytes/second. Returns `0` if unlimited.

---

### Set Global Download Limit

**Endpoint:** `POST /api/v2/transfer/setDownloadLimit`

**Parameters:**
| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | int | Download speed limit in bytes/second. Use `0` for unlimited. |

**Response:** HTTP 200 (no body)

---

### Alternative Speed Limits Mode

qBittorrent supports "alternative speed limits" which can be toggled on/off.

**Get Current Mode:**
- **Endpoint:** `GET /api/v2/transfer/speedLimitsMode`
- **Response:** `1` if alternative limits enabled, `0` if disabled

**Toggle Mode:**
- **Endpoint:** `POST /api/v2/transfer/toggleSpeedLimitsMode`
- **Response:** HTTP 200

---

## Gotchas and Important Notes

### 1. CSRF Protection (Referer/Origin Header)

**Critical:** qBittorrent requires the `Referer` or `Origin` header to match the request URL's domain and port for CSRF protection.

```python
# WRONG - Will fail with 401/403
requests.post("http://localhost:8080/api/v2/auth/login", data={...})

# CORRECT - Include Referer header
requests.post(
    "http://localhost:8080/api/v2/auth/login",
    data={...},
    headers={"Referer": "http://localhost:8080"}
)
```

### 2. HTTP Method Matters

As of qBittorrent v4.4.4, using the wrong HTTP method returns **405 Method Not Allowed**:
- Use `GET` for retrieving data
- Use `POST` for modifying state (setting limits, login, logout)

### 3. Speed Limits in Bytes per Second

All speed limits are in **bytes per second**, not bits:
- 1 MB/s = 1,048,576 bytes/s (1024 * 1024)
- 10 MB/s = 10,485,760 bytes/s
- Use `0` to remove the limit (unlimited)

### 4. Session Cookie Management

- The SID cookie expires after inactivity
- Always handle re-authentication if you receive 403 responses
- The `qbittorrent-api` Python library handles this automatically

### 5. IP Ban on Failed Logins

Multiple failed login attempts will result in IP banning. The WebUI will return 403 Forbidden.

### 6. Reverse Proxy Considerations

If using a reverse proxy (nginx, etc.), you may need to:
- Disable CSRF protection in qBittorrent settings (Options -> WebUI -> Enable Cross-Site Request Forgery protection)
- Or properly forward/set the Origin/Referer headers

---

## Python Code Examples

### Using Raw `requests` Library

```python
import requests

class QBittorrentClient:
    def __init__(self, host: str, port: int, username: str, password: str):
        self.base_url = f"http://{host}:{port}"
        self.session = requests.Session()
        self._login(username, password)

    def _login(self, username: str, password: str) -> None:
        """Authenticate and store session cookie."""
        url = f"{self.base_url}/api/v2/auth/login"
        response = self.session.post(
            url,
            data={"username": username, "password": password},
            headers={"Referer": self.base_url}
        )
        if response.status_code == 403:
            raise Exception("Login failed: IP might be banned")
        if "SID" not in self.session.cookies:
            raise Exception("Login failed: No session cookie received")

    def get_transfer_info(self) -> dict:
        """Get global transfer statistics."""
        url = f"{self.base_url}/api/v2/transfer/info"
        response = self.session.get(url)
        response.raise_for_status()
        return response.json()

    def get_upload_limit(self) -> int:
        """Get current global upload limit in bytes/s (0 = unlimited)."""
        url = f"{self.base_url}/api/v2/transfer/uploadLimit"
        response = self.session.get(url)
        response.raise_for_status()
        return int(response.text)

    def set_upload_limit(self, limit_bytes: int) -> None:
        """Set global upload limit in bytes/s (0 = unlimited)."""
        url = f"{self.base_url}/api/v2/transfer/setUploadLimit"
        response = self.session.post(url, data={"limit": limit_bytes})
        response.raise_for_status()

    def get_download_limit(self) -> int:
        """Get current global download limit in bytes/s (0 = unlimited)."""
        url = f"{self.base_url}/api/v2/transfer/downloadLimit"
        response = self.session.get(url)
        response.raise_for_status()
        return int(response.text)

    def set_download_limit(self, limit_bytes: int) -> None:
        """Set global download limit in bytes/s (0 = unlimited)."""
        url = f"{self.base_url}/api/v2/transfer/setDownloadLimit"
        response = self.session.post(url, data={"limit": limit_bytes})
        response.raise_for_status()

    def logout(self) -> None:
        """End the session."""
        url = f"{self.base_url}/api/v2/auth/logout"
        self.session.post(url)


# Usage example
if __name__ == "__main__":
    client = QBittorrentClient(
        host="localhost",
        port=8080,
        username="admin",
        password="adminadmin"
    )

    # Get current transfer info
    info = client.get_transfer_info()
    print(f"Current upload speed: {info['up_info_speed'] / 1024:.2f} KB/s")
    print(f"Current upload limit: {info['up_rate_limit'] / 1024:.2f} KB/s")

    # Set upload limit to 5 MB/s
    client.set_upload_limit(5 * 1024 * 1024)
    print("Upload limit set to 5 MB/s")

    # Remove upload limit
    client.set_upload_limit(0)
    print("Upload limit removed (unlimited)")

    client.logout()
```

### Using `qbittorrent-api` Library (Recommended)

The `qbittorrent-api` library provides a more robust, feature-complete interface with automatic session handling.

**Installation:**
```bash
pip install qbittorrent-api
```

**Usage:**
```python
import qbittorrentapi

# Connect to qBittorrent
client = qbittorrentapi.Client(
    host="localhost",
    port=8080,
    username="admin",
    password="adminadmin"
)

# Authentication is handled automatically
# The library re-authenticates if the session expires

# Get transfer info
info = client.transfer_info()
print(f"Upload speed: {info.up_info_speed / 1024:.2f} KB/s")
print(f"Download speed: {info.dl_info_speed / 1024:.2f} KB/s")
print(f"Upload limit: {info.up_rate_limit / 1024:.2f} KB/s")
print(f"Download limit: {info.dl_rate_limit / 1024:.2f} KB/s")

# Get current upload limit
current_limit = client.transfer_upload_limit()
print(f"Current upload limit: {current_limit} bytes/s")

# Set upload limit to 10 MB/s
client.transfer_set_upload_limit(limit=10 * 1024 * 1024)

# Alternative: Use property-style access
client.transfer.upload_limit = 10 * 1024 * 1024

# Remove limit (set to unlimited)
client.transfer_set_upload_limit(limit=0)

# Toggle alternative speed limits mode
client.transfer_toggle_speed_limits_mode()

# Check if alternative mode is enabled
mode = client.transfer_speed_limits_mode()
print(f"Alternative speed limits: {'enabled' if mode == '1' else 'disabled'}")
```

---

## Quick Reference

| Action | Method | Endpoint |
|--------|--------|----------|
| Login | POST | `/api/v2/auth/login` |
| Logout | POST | `/api/v2/auth/logout` |
| Get transfer info | GET | `/api/v2/transfer/info` |
| Get upload limit | GET | `/api/v2/transfer/uploadLimit` |
| Set upload limit | POST | `/api/v2/transfer/setUploadLimit` |
| Get download limit | GET | `/api/v2/transfer/downloadLimit` |
| Set download limit | POST | `/api/v2/transfer/setDownloadLimit` |
| Get alt speed mode | GET | `/api/v2/transfer/speedLimitsMode` |
| Toggle alt speed mode | POST | `/api/v2/transfer/toggleSpeedLimitsMode` |

---

## Sources

- [qBittorrent WebUI API (v4.1)](https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-4.1))
- [qBittorrent WebUI API (v5.0)](https://github.com/qbittorrent/qBittorrent/wiki/WebUI-API-(qBittorrent-5.0))
- [qbittorrent-api Python library](https://qbittorrent-api.readthedocs.io/)
- [qbittorrent-api Transfer methods](https://qbittorrent-api.readthedocs.io/en/latest/apidoc/transfer.html)
- [qbittorrent-api on PyPI](https://pypi.org/project/qbittorrent-api/)
