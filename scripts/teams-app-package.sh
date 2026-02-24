#!/usr/bin/env bash
# teams-app-package.sh — Generate a Microsoft Teams app package for OpenIncident.
#
# Usage:
#   ./scripts/teams-app-package.sh
#   TEAMS_APP_ID=<id> ./scripts/teams-app-package.sh
#
# The script reads TEAMS_APP_ID from the environment. If not set it falls back
# to .env in the repo root. Everything else is derived or generated automatically.
#
# Output: openincident-teams-app.zip (ready to sideload in Teams)

set -euo pipefail

# ── Resolve repo root ────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ── Load TEAMS_APP_ID ────────────────────────────────────────────────────────
if [ -z "${TEAMS_APP_ID:-}" ] && [ -f "$REPO_ROOT/.env" ]; then
    TEAMS_APP_ID="$(grep '^TEAMS_APP_ID=' "$REPO_ROOT/.env" 2>/dev/null | cut -d= -f2 | tr -d '[:space:]')"
fi

if [ -z "${TEAMS_APP_ID:-}" ]; then
    echo "Error: TEAMS_APP_ID is not set."
    echo ""
    echo "Set it in .env:"
    echo "  TEAMS_APP_ID=<your-azure-app-registration-id>"
    echo ""
    echo "Or export it:"
    echo "  TEAMS_APP_ID=<id> make teams-app-package"
    exit 1
fi

# ── Check Python 3 ───────────────────────────────────────────────────────────
if ! command -v python3 &>/dev/null; then
    echo "Error: python3 is required to generate the app package."
    exit 1
fi

# ── Generate a fresh GUID for the Teams app ID ───────────────────────────────
# The Teams app ID must be a DIFFERENT GUID from the Bot App ID (TEAMS_APP_ID).
# The bot app ID lives in bots[0].botId; the top-level id is the Teams app identity.
TEAMS_APP_GUID="$(python3 -c "import uuid; print(uuid.uuid4())")"

OUTPUT="$REPO_ROOT/openincident-teams-app.zip"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# ── Write manifest.json ───────────────────────────────────────────────────────
cat > "$TMP/manifest.json" << MANIFEST
{
  "\$schema": "https://developer.microsoft.com/en-us/json-schemas/teams/v1.11/MicrosoftTeams.schema.json",
  "manifestVersion": "1.11",
  "version": "1.0.0",
  "id": "${TEAMS_APP_GUID}",
  "packageName": "com.openincident.bot",
  "developer": {
    "name": "OpenIncident",
    "websiteUrl": "https://github.com/openincident/openincident",
    "privacyUrl": "https://github.com/openincident/openincident/blob/main/docs/PRIVACY.md",
    "termsOfUseUrl": "https://github.com/openincident/openincident/blob/main/LICENSE"
  },
  "icons": {
    "color": "color.png",
    "outline": "outline.png"
  },
  "name": {
    "short": "OpenIncident",
    "full": "OpenIncident Bot"
  },
  "description": {
    "short": "Incident management bot",
    "full": "OpenIncident bot — acknowledge, resolve, and track incidents directly from Teams. Type @OpenIncident ack, resolve, status, or new."
  },
  "accentColor": "#E53E3E",
  "bots": [
    {
      "botId": "${TEAMS_APP_ID}",
      "scopes": ["team", "personal"],
      "supportsFiles": false,
      "isNotificationOnly": false,
      "commandLists": [
        {
          "scopes": ["team"],
          "commands": [
            { "title": "ack",     "description": "Acknowledge the active incident" },
            { "title": "resolve", "description": "Resolve the active incident" },
            { "title": "status",  "description": "Show current incident status" },
            { "title": "new",     "description": "Create a new incident" }
          ]
        }
      ]
    }
  ],
  "permissions": ["identity", "messageTeamMembers"],
  "validDomains": []
}
MANIFEST

# ── Generate icons ────────────────────────────────────────────────────────────
# Pure-Python PNG generation — no Pillow required. Both icons meet Teams specs:
#   color.png  : 192x192, full colour (red with white "OI" initials)
#   outline.png: 32x32,   white on transparent background
TMP_DIR="$TMP" python3 << 'PYEOF'
import struct, zlib, math, os

TMP = os.environ.get("TMP_DIR")

# ── Low-level PNG writer ──────────────────────────────────────────────────────
def _chunk(tag: bytes, data: bytes) -> bytes:
    crc = zlib.crc32(tag + data) & 0xFFFFFFFF
    return struct.pack(">I", len(data)) + tag + data + struct.pack(">I", crc)

def write_png(path: str, width: int, height: int, pixels):
    """pixels: list[list[(r,g,b,a)]] — rows of (r,g,b,a) tuples."""
    raw = b""
    for row in pixels:
        raw += b"\x00"  # filter byte: None
        for r, g, b, a in row:
            raw += bytes([r, g, b, a])
    ihdr = struct.pack(">IIBBBBB", width, height, 8, 6, 0, 0, 0)  # 8-bit RGBA
    idat = zlib.compress(raw, 9)
    with open(path, "wb") as f:
        f.write(b"\x89PNG\r\n\x1a\n")
        f.write(_chunk(b"IHDR", ihdr))
        f.write(_chunk(b"IDAT", idat))
        f.write(_chunk(b"IEND", b""))

# ── color.png: 192x192 red background with white "OI" drawn in pixels ────────
W, H = 192, 192
RED   = (229, 62, 62, 255)   # #E53E3E
WHITE = (255, 255, 255, 255)

# Minimal 5×7 bitmap font for "OI" (each char is a list of 5-wide rows)
GLYPHS = {
    "O": [
        [0,1,1,1,0],
        [1,0,0,0,1],
        [1,0,0,0,1],
        [1,0,0,0,1],
        [1,0,0,0,1],
        [1,0,0,0,1],
        [0,1,1,1,0],
    ],
    "I": [
        [1,1,1,1,1],
        [0,0,1,0,0],
        [0,0,1,0,0],
        [0,0,1,0,0],
        [0,0,1,0,0],
        [0,0,1,0,0],
        [1,1,1,1,1],
    ],
}

def draw_glyph(grid, glyph, ox, oy, scale, colour):
    for gy, row in enumerate(GLYPHS[glyph]):
        for gx, on in enumerate(row):
            if on:
                for dy in range(scale):
                    for dx in range(scale):
                        y, x = oy + gy * scale + dy, ox + gx * scale + dx
                        if 0 <= y < len(grid) and 0 <= x < len(grid[0]):
                            grid[y][x] = colour

grid = [[RED] * W for _ in range(H)]
SCALE = 8
# Centre "OI": two glyphs each 5 wide, gap of 1 col, total = 5+1+5 = 11 cols * scale
total_w = (5 + 1 + 5) * SCALE
total_h = 7 * SCALE
ox = (W - total_w) // 2
oy = (H - total_h) // 2
draw_glyph(grid, "O", ox,                  oy, SCALE, WHITE)
draw_glyph(grid, "I", ox + (5 + 1) * SCALE, oy, SCALE, WHITE)

write_png(f"{TMP}/color.png", W, H, grid)

# ── outline.png: 32x32 white circle on transparent background ─────────────────
OW, OH = 32, 32
TRANSPARENT = (0, 0, 0, 0)
ogrid = [[TRANSPARENT] * OW for _ in range(OH)]
cx, cy, r = OW / 2, OH / 2, 13.0
for y in range(OH):
    for x in range(OW):
        dist = math.sqrt((x + 0.5 - cx) ** 2 + (y + 0.5 - cy) ** 2)
        if dist <= r:
            ogrid[y][x] = WHITE

write_png(f"{TMP}/outline.png", OW, OH, ogrid)
print("icons generated")
PYEOF

# ── Zip the package ───────────────────────────────────────────────────────────
(cd "$TMP" && zip -q -j "$OUTPUT" manifest.json color.png outline.png)

echo ""
echo "Teams app package created: openincident-teams-app.zip"
echo ""
echo "Bot App ID : $TEAMS_APP_ID"
echo "Teams App ID (manifest): $TEAMS_APP_GUID"
echo ""
echo "Next steps:"
echo ""
echo "  1. Open Microsoft Teams"
echo "  2. Go to your incident-management team"
echo "  3. Click ··· next to the team name -> Manage team -> Apps tab"
echo "  4. Click 'Upload a custom app' -> select openincident-teams-app.zip"
echo "  5. Click Add -> Add to a team -> select your team -> Set up a bot"
echo ""
echo "Once installed, create an incident to test:"
echo "  curl -X POST http://localhost:8080/api/v1/incidents \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"title\":\"Test incident\",\"severity\":\"high\"}'"
echo ""
