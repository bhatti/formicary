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
    get("/api/v1/jobs/definitions")

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

    # Mermaid diagram of the job definition
    get(f"/api/v1/jobs/definitions/{jd_id}/mermaid")

    # Update concurrency — body carries the new value
    put(f"/api/v1/jobs/definitions/{jd_id}/concurrency", {"concurrency": 2})

    # Stats — no ID, global stats endpoint
    get("/api/v1/jobs/definitions/stats")

    # Plugins listing
    get("/api/v1/jobs/plugins")

    # Disable / Enable
    post(f"/api/v1/jobs/definitions/{jd_id}/disable", {}, expected=(200,))
    post(f"/api/v1/jobs/definitions/{jd_id}/enable", {}, expected=(200,))

    return unique, jd_id


def test_job_configs(job_id):
    section("Job Definition Configs (CRUD)")
    if not job_id:
        print(f"  [{SKIP}] No job_id — skipping job configs tests")
        return

    # List configs for job
    get(f"/api/v1/jobs/{job_id}/configs")

    # Create a config
    cfg = {
        "name": f"smoke_cfg_{uuid.uuid4().hex[:6]}",
        "value": "smoke_value",
        "secret": False,
    }
    ok, _, body = post(f"/api/v1/jobs/{job_id}/configs", cfg)
    if not ok:
        print(f"  [{SKIP}] Config create failed — skipping further config tests")
        return

    d = parse_body(body)
    cfg_id = d.get("id", "")
    if not cfg_id:
        print(f"  [{SKIP}] No config ID in create response")
        return

    # Read config
    get(f"/api/v1/jobs/{job_id}/configs/{cfg_id}")

    # Update config
    put(f"/api/v1/jobs/{job_id}/configs/{cfg_id}", {"name": cfg["name"], "value": "updated_value", "secret": False})

    # Delete config
    delete(f"/api/v1/jobs/{job_id}/configs/{cfg_id}")


def test_job_request_lifecycle():
    section("Job Request Lifecycle (submit → get → mermaid → cancel)")

    # List requests
    get("/api/v1/jobs/requests")
    get("/api/v1/jobs/requests?page=0&page_size=10")
    get("/api/v1/jobs/requests/stats")

    # Submit a new request — requires a registered job_type to exist
    # Use a unique type that won't be found so we can test 404 handling gracefully
    unique = f"smoke-req-{uuid.uuid4().hex[:8]}"
    req_body = {
        "job_type": unique,
        "description": "Smoke test job request",
        "job_group": "smoke",
        "job_priority": 1,
        "params": {"key1": "value1"},
    }
    # Expect 404 because job_type won't be registered; 201 if it happens to match
    ok, status, body = post("/api/v1/jobs/requests", req_body, expected=(201, 404, 500))
    if ok and status == 201:
        d = parse_body(body)
        req_id = d.get("id", "")
        if req_id:
            # Get the specific request
            get(f"/api/v1/jobs/requests/{req_id}")

            # Wait time estimate
            get(f"/api/v1/jobs/requests/{req_id}/wait_time")

            # Mermaid graph for request
            get(f"/api/v1/jobs/requests/{req_id}/mermaid")

            # Cancel the request
            post(f"/api/v1/jobs/requests/{req_id}/cancel", {}, expected=(200, 400, 422))


def test_job_request_controls():
    section("Job Request Controls (pause/restart/trigger/review)")
    # List current requests to find one to test against
    ok, _, body = get("/api/v1/jobs/requests")
    if not ok:
        return

    d = parse_body(body)
    records = d.get("records", [])
    if not records:
        print(f"  [{SKIP}] No existing job requests to test controls against")
        return

    req_id = records[0].get("id", "")
    if not req_id:
        return

    # These will return errors if state is wrong, which is expected
    post(f"/api/v1/jobs/requests/{req_id}/pause", {}, expected=(200, 400, 422))
    post(f"/api/v1/jobs/requests/{req_id}/restart", {}, expected=(200, 400, 422))
    post(f"/api/v1/jobs/requests/{req_id}/trigger", {}, expected=(200, 400, 422))
    # Review requires approved/rejected body; test with reject
    post(f"/api/v1/jobs/requests/{req_id}/review",
         {"status": "REJECTED", "comments": "smoke test"},
         expected=(200, 400, 404, 422))


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
    section("Organizations (CRUD)")
    ok, _, body = get("/api/v1/orgs")
    if ok:
        d = parse_body(body)
        records = d.get("records", [])
        if records:
            org_id = records[0].get("id", "")
            if org_id:
                get(f"/api/v1/orgs/{org_id}")

    # Create org — requires admin
    unique = f"smoke-org-{uuid.uuid4().hex[:8]}"
    org_body = {
        "org_unit": unique,
        "bundle_id": f"com.smoke.{unique}",
    }
    ok, _, body = post("/api/v1/orgs", org_body, expected=(200, 201, 403, 422))
    if ok and parse_body(body).get("id"):
        org_id = parse_body(body)["id"]
        # Update
        put(f"/api/v1/orgs/{org_id}", {**org_body, "id": org_id}, expected=(200, 403))
        # Delete
        delete(f"/api/v1/orgs/{org_id}", expected=(200, 204, 403))


def test_org_configs():
    section("Organization Configs")
    ok, _, body = get("/api/v1/orgs")
    if not ok:
        return

    d = parse_body(body)
    records = d.get("records", [])
    if not records:
        print(f"  [{SKIP}] No orgs to test org configs against")
        return

    org_id = records[0].get("id", "")
    if not org_id:
        return

    # List org configs
    get(f"/api/v1/orgs/{org_id}/configs")

    # Create org config
    cfg = {
        "name": f"smoke_org_cfg_{uuid.uuid4().hex[:6]}",
        "value": "smoke_org_value",
        "secret": False,
    }
    ok, _, body = post(f"/api/v1/orgs/{org_id}/configs", cfg, expected=(200, 201, 403))
    if ok and parse_body(body).get("id"):
        cfg_id = parse_body(body)["id"]
        # Read
        get(f"/api/v1/orgs/{org_id}/configs/{cfg_id}")
        # Delete
        delete(f"/api/v1/orgs/{org_id}/configs/{cfg_id}", expected=(200, 204, 403))


def test_artifacts():
    section("Artifacts")
    ok, _, body = get("/api/v1/artifacts")
    if ok:
        d = parse_body(body)
        records = d.get("records", [])
        if records:
            artifact_id = records[0].get("id", "")
            if artifact_id:
                get(f"/api/v1/artifacts/{artifact_id}")


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
    section("System Configs (CRUD)")
    ok, _, body = get("/api/v1/configs")
    if ok:
        d = parse_body(body)
        records = d.get("records", [])
        if records:
            cfg_id = records[0].get("id", "")
            if cfg_id:
                get(f"/api/v1/configs/{cfg_id}")
                # Delete requires admin; tolerate 403
                delete(f"/api/v1/configs/{cfg_id}", expected=(200, 204, 403))


def test_resources():
    section("Job Resources (CRUD)")
    ok, _, body = get("/api/v1/resources")
    if ok:
        d = parse_body(body)
        records = d.get("records", [])
        if records:
            res_id = records[0].get("id", "")
            if res_id:
                get(f"/api/v1/resources/{res_id}")


def test_ant_registrations():
    section("Ant Registrations")
    ok, _, body = get("/api/v1/ants")
    if ok:
        d = parse_body(body)
        records = d.get("records", [])
        if records:
            ant_id = records[0].get("ant_id", "") or records[0].get("id", "")
            if ant_id:
                get(f"/api/v1/ants/{ant_id}")


def test_subscriptions():
    section("Subscriptions (CRUD)")
    ok, _, body = get("/api/v1/subscriptions")
    if ok:
        d = parse_body(body)
        records = d.get("records", [])
        if records:
            sub_id = records[0].get("id", "")
            if sub_id:
                get(f"/api/v1/subscriptions/{sub_id}")

    # Create subscription — requires admin
    sub_body = {
        "kind": "BASIC",
        "policy": "MONTHLY",
        "period": "MONTH",
        "price": 0,
        "cpu_quota": 3600,
        "disk_quota": 1000,
        "active": True,
    }
    ok, _, body = post("/api/v1/subscriptions", sub_body, expected=(200, 201, 403))
    if ok and parse_body(body).get("id"):
        sub_id = parse_body(body)["id"]
        # Update
        put(f"/api/v1/subscriptions/{sub_id}", {**sub_body, "id": sub_id}, expected=(200, 403))
        # Delete
        delete(f"/api/v1/subscriptions/{sub_id}", expected=(200, 204, 403))


def test_audit():
    section("Audit Records")
    get("/api/v1/audit")


def test_admin():
    section("Admin / Dashboard Stats")
    get("/api/v1/admin/dashboard")


def test_login():
    section("Login (requires authenticated session)")
    # Login re-issues a JWT for an already-authenticated caller via OAuth.
    # Auth disabled: anonymous user not in DB → 404 (user not found).
    # Auth enabled with valid Bearer: returns 200 with fresh token.
    post("/api/v1/auth/login", {}, expected=(200, 404, 401))


def test_job_definition_cleanup(jd_id):
    """Delete the job definition created in test_job_definitions."""
    if jd_id:
        section("Job Definition Cleanup")
        delete(f"/api/v1/jobs/definitions/{jd_id}")


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

    # Job definitions CRUD — returns (job_type, jd_id) or None
    jd_result = test_job_definitions()
    jd_id = jd_result[1] if jd_result else None
    job_type = jd_result[0] if jd_result else None

    # Job definition configs — requires a valid job definition ID
    test_job_configs(jd_id)

    # Job requests
    test_job_request_lifecycle()
    test_job_request_controls()

    # Users, orgs
    test_users()
    test_organizations()
    test_org_configs()

    # Artifacts, error codes, configs
    test_artifacts()
    test_error_codes()
    test_configs()

    # Resources and subscriptions
    test_resources()
    test_ant_registrations()
    test_subscriptions()

    # Audit and admin
    test_audit()
    test_admin()

    # Auth
    test_login()

    # Cleanup — delete job definition created above
    test_job_definition_cleanup(jd_id)

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
