import datetime
import json
import math
import os
import sys
import urllib.error
import urllib.request

# shortcut: file credentials only; add Keychain support when a remote host needs it.
try:
    config = os.environ.get("CLAUDE_CONFIG_DIR") or os.path.expanduser("~/.claude")
    with open(os.path.join(config, ".credentials.json"), "rb") as source:
        credential = json.loads(source.read(2 * 1024 * 1024))
    token = credential["claudeAiOauth"]["accessToken"]
    if not isinstance(token, str) or not token:
        sys.exit(20)
    request = urllib.request.Request(
        "https://api.anthropic.com/api/oauth/usage",
        headers={
            "Authorization": "Bearer " + token,
            "anthropic-beta": "oauth-2025-04-20",
            "anthropic-version": "2023-06-01",
            "x-app": "cli",
            "User-Agent": "claude-code",
            "Accept": "application/json",
        },
    )
    # Do not forward authorization to a redirect destination.
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            return None

    with urllib.request.build_opener(NoRedirect).open(request, timeout=20) as response:
        raw = response.read(2 * 1024 * 1024 + 1)
    if len(raw) > 2 * 1024 * 1024:
        sys.exit(21)
    usage = json.loads(raw)
    result = {}
    for key in ("five_hour", "seven_day", "seven_day_sonnet", "seven_day_opus"):
        window = usage.get(key)
        if window is None:
            continue
        value = window.get("utilization")
        if value is None:
            continue
        if type(value) not in (int, float) or not math.isfinite(value) or not 0 <= value <= 100:
            sys.exit(21)
        reset = window.get("resets_at")
        if reset is not None:
            parsed = datetime.datetime.fromisoformat(reset.replace("Z", "+00:00"))
            if parsed.tzinfo is None:
                sys.exit(21)
            reset = parsed.isoformat()
        result[key] = {"utilization": value, "resets_at": reset}
    if not result:
        sys.exit(21)
    print(json.dumps(result, allow_nan=False))
except urllib.error.HTTPError as error:
    sys.exit(20 if error.code in (401, 403) else 22 if error.code == 429 else 23)
except (FileNotFoundError, PermissionError, KeyError):
    sys.exit(20)
except (ValueError, TypeError, AttributeError):
    sys.exit(21)
except Exception:
    sys.exit(23)
