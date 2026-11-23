"""ECS canary smoke test, run as a POST_TEST_TRAFFIC_SHIFT lifecycle hook.

Reads the test URL from the hook's details and returns SUCCEEDED on a 2xx.
Every error returns FAILED, so a crash can't be mistaken for a missing status.
"""
import json
import time
import urllib.error
import urllib.request

ATTEMPTS = 3
DELAY_SECONDS = 3
TIMEOUT_SECONDS = 5
TEST_HEADER = "x-amzn-ecs-blue-green-test"


def lambda_handler(event, context):
    print(json.dumps(event))
    try:
        url = test_url(event)
        for attempt in range(1, ATTEMPTS + 1):
            status = get_status(url)
            print(f"attempt {attempt}: {url} -> {status}")
            if 200 <= status < 300:
                return {"hookStatus": "SUCCEEDED"}
            if attempt < ATTEMPTS:
                time.sleep(DELAY_SECONDS)
    except Exception as err:
        print(f"smoke test error: {err!r}")
    return {"hookStatus": "FAILED"}


def test_url(event):
    details = event.get("hookDetails") or {}
    if isinstance(details, str):
        details = json.loads(details)
    url = details.get("url")
    if not url:
        raise ValueError("no url in hookDetails")
    return url


def get_status(url):
    req = urllib.request.Request(url, headers={TEST_HEADER: "1"})
    try:
        with urllib.request.urlopen(req, timeout=TIMEOUT_SECONDS) as resp:
            return resp.status
    except urllib.error.HTTPError as err:
        return err.code
    except (urllib.error.URLError, TimeoutError) as err:
        print(f"request failed: {err!r}")
        return 0
