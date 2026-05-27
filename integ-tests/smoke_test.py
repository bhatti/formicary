#!/usr/bin/env python3
"""
Smoke test for the Formicary /api/v1/* gRPC-gateway REST endpoints.

Usage:
    # Auth disabled (default):
    python smoke_test.py

    # Auth enabled (provide a JWT token):
    TOKEN=<jwt> python smoke_test.py

    # Custom server:
    BASE_URL=http://localhost:7777 python smoke_test.py

    # Verbose (show response bodies):
    VERBOSE=1 python smoke_test.py

Exit code 0 = all tests passed, non-zero = failures.
"""

import json
import os
import sys
import time
import uuid

try:
    import requests
except ImportError:
    print("ERROR: 'requests' not installed. Run: pip install requests")
    sys.exit(1)

BASE_URL = os.environ.get("BASE_URL", "http://localhost:7777").rstrip("/")
TOKEN = os.environ.get("TOKEN", "")
VERBOSE = os.environ.get("VERBOSE", "").lower() in ("1", "true", "yes")

# ---- helpers ----------------------------------------------------------------

PASS = "\033[32mPASS\033[0m"
FAIL = "\033[31mFAIL\033[0m"
SKIP = "\033[33mSKIP\033[0m"

results = []


def headers():
    h = {"Content-Type": "application/json", "Accept": "application/json"}
    if TOKEN:
        h["Authorization"] = f"Bearer {TOKEN}"
    return h


def _log(label, method, url, status, body, expected):
    ok = status in expected
    tag = PASS if ok else FAIL
    print(f"  [{tag}] {method} {url}  →  HTTP {status}")
    if VERBOSE or not ok:
        try:
            parsed = json.loads(body)
            print(f"         {json.dumps(parsed, indent=2)[:400]}")
        except Exception:
            print(f"         {body[:200]}")
    results.append((ok, f"{method} {url}", status))
    return ok, status, body


def get(path, expected=(200,)):
    url = BASE_URL + path
    try:
        r = requests.get(url, headers=headers(), timeout=10)
        return _log("GET", "GET", url, r.status_code, r.text, expected)
    except Exception as e:
        print(f"  [{FAIL}] GET {url}  →  EXCEPTION: {e}")
        results.append((False, f"GET {url}", "exception"))
        return False, 0, ""


def post(path, body, expected=(200, 201)):
    url = BASE_URL + path
    try:
        r = requests.post(url, headers=headers(), json=body, timeout=10)
        return _log("POST", "POST", url, r.status_code, r.text, expected)
    except Exception as e:
        print(f"  [{FAIL}] POST {url}  →  EXCEPTION: {e}")
        results.append((False, f"POST {url}", "exception"))
        return False, 0, ""


def put(path, body, expected=(200,)):
    url = BASE_URL + path
    try:
        r = requests.put(url, headers=headers(), json=body, timeout=10)
        return _log("PUT", "PUT", url, r.status_code, r.text, expected)
    except Exception as e:
        print(f"  [{FAIL}] PUT {url}  →  EXCEPTION: {e}")
        results.append((False, f"PUT {url}", "exception"))
        return False, 0, ""


def delete(path, expected=(200, 204)):
    url = BASE_URL + path
    try:
        r = requests.delete(url, headers=headers(), timeout=10)
        return _log("DELETE", "DELETE", url, r.status_code, r.text, expected)
    except Exception as e:
        print(f"  [{FAIL}] DELETE {url}  →  EXCEPTION: {e}")
        results.append((False, f"DELETE {url}", "exception"))
        return False, 0, ""


def section(name):
    print(f"\n{'=' * 60}")
    print(f"  {name}")
    print(f"{'=' * 60}")


def parse_body(body):
    try:
        return json.loads(body)
    except Exception:
        return {}


# ---- test cases -------------------------------------------------------------

def test_health():
    section("Health / Ping")
    get("/api/v1/ping", expected=(200,))
    get("/api/v1/health", expected=(200,))


def test_job_definitions():
    section("Job Definitions (CRUD)")
    unique = f"smoke-{uuid.uuid4().hex[:8]}"

    # List
    ok, _, body = get("/api/v1/jobs/definitions")

    # Create — wrap in the request envelope that proto/grpc-gateway expects.
    # raw_yaml is required by the job definition validator.
    raw_yaml = f"job_type: {unique}\ndescription: Smoke test job\ntasks:\n- task_type: step1\n  method: SHELL\n  script:\n    - echo hello\n"
    jd = {
        "job_definition": {
            "job_type": unique,
            "description": "Smoke test job",
            "platform": "LINUX",
            "retry": 0,
            "sem_version": "1.0.0",
            "raw_yaml": raw_yaml,
            "tasks": [
                {
                    "task_type": "step1",
                    "method": "SHELL",
                    "script": ["echo hello"],
                }
            ],
        }
    }
    ok, status, body = post("/api/v1/jobs/definitions", jd)
    if not ok:
        print(f"  [{SKIP}] Skipping read/update/delete — create failed")
        return None

    d = parse_body(body)
    jd_id = d.get("job_definition", {}).get("id") or d.get("id", "")
    if not jd_id:
        print(f"  [{SKIP}] No ID in create response — skipping further CRUD")
        return None

    # Read
    get(f"/api/v1/jobs/definitions/{jd_id}")

    # YAML view — uses job_type, not ID
    get(f"/api/v1/jobs/definitions/{unique}/yaml")

    # Update concurrency — body carries the new value
    put(f"/api/v1/jobs/definitions/{jd_id}/concurrency", {"concurrency": 2})

    # Stats — no ID, global stats endpoint
    get("/api/v1/jobs/definitions/stats")

    # Disable / Enable
    post(f"/api/v1/jobs/definitions/{jd_id}/disable", {}, expected=(200,))
    post(f"/api/v1/jobs/definitions/{jd_id}/enable", {}, expected=(200,))

    # Delete
    delete(f"/api/v1/jobs/definitions/{jd_id}")

    return unique


def test_job_requests():
    section("Job Requests / Executions")
    get("/api/v1/jobs/requests")
    get("/api/v1/jobs/requests?page=0&page_size=10")
    get("/api/v1/jobs/requests/stats")
    # wait_time requires a specific request ID — just verify the pattern exists, skip bare endpoint


def test_users():
    section("Users")
    ok, _, body = get("/api/v1/users")
    if ok:
        d = parse_body(body)
        records = d.get("records", [])
        if records:
            uid = records[0].get("id", "")
            if uid:
                get(f"/api/v1/users/{uid}")
    # profile returns 404 when the anonymous admin user isn't in the DB
    get("/api/v1/users/profile", expected=(200, 404))


def test_organizations():
    section("Organizations")
    get("/api/v1/orgs")


def test_artifacts():
    section("Artifacts")
    get("/api/v1/artifacts")


def test_error_codes():
    section("Error Codes (CRUD)")
    get("/api/v1/error-codes")

    unique = f"ERR_SMOKE_{uuid.uuid4().hex[:6].upper()}"
    # body: "error_code" binding — request body IS the ErrorCode object directly
    ec = {
        "error_code": unique,
        "description": "Smoke test error code",
        "regex": unique,
        "action": "RETRY",
        "retry_delay": 5,
        "hard_failure": False,
        "can_retry": True,
    }
    ok, _, body = post("/api/v1/error-codes", ec)
    if ok:
        d = parse_body(body)
        ec_id = d.get("error_code", {}).get("id") or d.get("id", "")
        if ec_id:
            get(f"/api/v1/error-codes/{ec_id}")
            delete(f"/api/v1/error-codes/{ec_id}")


def test_configs():
    section("System Configs")
    get("/api/v1/configs")


def test_resources():
    section("Resources (Ant Registrations / Subscriptions)")
    get("/api/v1/ants")
    get("/api/v1/subscriptions")


def test_audit():
    section("Audit Records")
    get("/api/v1/audit")


def test_admin():
    section("Admin / Dashboard Stats")
    get("/api/v1/admin/dashboard")


def test_job_resources():
    section("Job Resources")
    get("/api/v1/resources")


def test_login():
    section("Login (requires authenticated session)")
    # Login re-issues a JWT for an already-authenticated caller via OAuth.
    # Auth disabled: anonymous user not in DB → 404 (user not found).
    # Auth enabled with valid Bearer: returns 200 with fresh token.
    post("/api/v1/auth/login", {}, expected=(200, 404, 401))


# ---- entrypoint -------------------------------------------------------------

def main():
    print(f"\nFormicary API v1 Smoke Test")
    print(f"  Base URL : {BASE_URL}")
    print(f"  Auth     : {'Bearer token' if TOKEN else 'disabled (anonymous)'}")
    print(f"  Verbose  : {VERBOSE}")

    # Check connectivity
    try:
        r = requests.get(f"{BASE_URL}/api/v1/ping", timeout=5)
        if r.status_code != 200:
            print(f"\nERROR: server at {BASE_URL} returned {r.status_code} for /api/v1/ping")
            sys.exit(1)
    except Exception as e:
        print(f"\nERROR: cannot reach {BASE_URL}: {e}")
        sys.exit(1)

    # Run all test groups
    test_health()
    test_job_definitions()
    test_job_requests()
    test_users()
    test_organizations()
    test_artifacts()
    test_error_codes()
    test_configs()
    test_resources()
    test_audit()
    test_admin()
    test_job_resources()
    test_login()

    # Summary
    passed = sum(1 for ok, _, _ in results if ok)
    failed = sum(1 for ok, _, _ in results if not ok)
    total = len(results)

    print(f"\n{'=' * 60}")
    print(f"  Results: {passed}/{total} passed, {failed} failed")
    print(f"{'=' * 60}")

    if failed:
        print("\nFailed tests:")
        for ok, name, status in results:
            if not ok:
                print(f"  {FAIL}  {name}  (status={status})")
        sys.exit(1)
    else:
        print(f"\nAll {total} tests passed.")
        sys.exit(0)


if __name__ == "__main__":
    main()
