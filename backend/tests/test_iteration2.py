"""Iteration 2 backend tests: /api/settings + persistence + regression smoke."""
import json
import os
import subprocess
import time
import pytest
import requests

BASE_URL = os.environ.get(
    "REACT_APP_BACKEND_URL",
    "https://e10f40b8-5ce6-4479-b4c6-6ca6e6e3e95e.preview.emergentagent.com",
).rstrip("/")
API = f"{BASE_URL}/api"
STATE_FILE = "/app/backend/state.json"
DEFAULT_OUTPUT = "/app/downloads"


@pytest.fixture(scope="module")
def client():
    s = requests.Session()
    s.headers.update({"Content-Type": "application/json"})
    return s


# Helper: reset output_dir back to default after tests
@pytest.fixture(scope="module", autouse=True)
def cleanup_output_dir(client):
    yield
    try:
        client.post(f"{API}/settings", json={"output_dir": DEFAULT_OUTPUT}, timeout=15)
    except Exception:
        pass


class TestHealthSmoke:
    def test_health(self, client):
        r = client.get(f"{API}/health", timeout=15)
        assert r.status_code == 200
        assert r.json()["status"] == "ok"


class TestSettings:
    def test_get_settings_default(self, client):
        r = client.get(f"{API}/settings", timeout=15)
        assert r.status_code == 200
        data = r.json()
        assert "output_dir" in data
        assert isinstance(data["output_dir"], str)
        assert len(data["output_dir"]) > 0

    def test_post_settings_creates_and_persists(self, client):
        new_dir = "/tmp/entropy-test-2"
        # cleanup
        try:
            import shutil
            shutil.rmtree(new_dir, ignore_errors=True)
        except Exception:
            pass

        r = client.post(f"{API}/settings", json={"output_dir": new_dir}, timeout=15)
        assert r.status_code == 200, r.text
        data = r.json()
        assert data["output_dir"] == new_dir
        # Directory was actually created
        assert os.path.isdir(new_dir), f"expected dir {new_dir} to exist on disk"

        # GET reflects new value
        r2 = client.get(f"{API}/settings", timeout=15)
        assert r2.status_code == 200
        assert r2.json()["output_dir"] == new_dir

        # Wait for debounced persist (250ms) then verify state.json
        time.sleep(1.0)
        with open(STATE_FILE) as f:
            snap = json.load(f)
        assert snap.get("settings", {}).get("output_dir") == new_dir

    def test_post_settings_empty_returns_400(self, client):
        r = client.post(f"{API}/settings", json={"output_dir": ""}, timeout=15)
        assert r.status_code == 400
        body = r.json()
        assert "error" in body
        assert "required" in body["error"].lower()

    def test_post_settings_unwritable_returns_400(self, client):
        # /proc is a virtual filesystem; cannot mkdir there
        r = client.post(f"{API}/settings", json={"output_dir": "/proc/test123"}, timeout=15)
        assert r.status_code == 400, r.text
        assert "error" in r.json()

    def test_config_reflects_changed_output_dir(self, client):
        # The previous test set output_dir to /tmp/entropy-test-2
        r = client.get(f"{API}/config", timeout=15)
        assert r.status_code == 200
        data = r.json()
        assert data.get("output_dir") == "/tmp/entropy-test-2"


class TestPersistence:
    """Create job, restart backend, verify job restored as failed/interrupted."""

    def test_persistence_across_restart(self, client):
        # Use an unresolvable URL so the job won't complete during the test window.
        payload = {
            "items": [{
                "url": "https://www.youtube.com/watch?v=NEVERRESOLVE123",
                "title": "TEST_persistence_interrupt",
            }],
            "options": {"format": "mp3", "bitrate": "192", "engine": "ytdlp"},
        }
        r = client.post(f"{API}/jobs", json=payload, timeout=30)
        assert r.status_code in (200, 201), r.text
        jid = r.json()["jobs"][0]["id"]

        # Give the persist loop time to flush to disk
        time.sleep(1.0)

        # state.json should contain our job
        with open(STATE_FILE) as f:
            snap = json.load(f)
        jobs_in_state = snap.get("jobs", []) or []
        ids_in_state = []
        for raw in jobs_in_state:
            if isinstance(raw, dict):
                ids_in_state.append(raw.get("id"))
            elif isinstance(raw, str):
                try:
                    ids_in_state.append(json.loads(raw).get("id"))
                except Exception:
                    pass
        assert jid in ids_in_state, f"job {jid} not found in state.json jobs: {ids_in_state}"

        # Restart backend via supervisor
        rc = subprocess.run(
            ["sudo", "supervisorctl", "restart", "backend"],
            capture_output=True, text=True, timeout=30,
        )
        assert rc.returncode == 0, f"supervisorctl restart failed: {rc.stderr}"

        # Wait for backend to come back
        deadline = time.time() + 30
        ok = False
        while time.time() < deadline:
            try:
                hr = requests.get(f"{API}/health", timeout=5)
                if hr.status_code == 200:
                    ok = True
                    break
            except Exception:
                pass
            time.sleep(1)
        assert ok, "backend did not come back online after restart"

        # GET /api/jobs and verify job is restored
        time.sleep(1.0)
        r = client.get(f"{API}/jobs", timeout=15)
        assert r.status_code == 200
        jobs = r.json()["jobs"]
        match = next((j for j in jobs if j["id"] == jid), None)
        assert match is not None, f"job {jid} not restored after restart"
        # Non-terminal -> failed with interrupted error
        assert match["status"] == "failed", f"expected failed, got {match['status']}"
        assert "interrupt" in (match.get("error", "") or "").lower(), match.get("error")

        # cleanup: remove the test job
        client.delete(f"{API}/jobs/{jid}", timeout=15)

    def test_settings_survive_restart(self, client):
        # After the previous restart, /api/settings should still report /tmp/entropy-test-2
        r = client.get(f"{API}/settings", timeout=15)
        assert r.status_code == 200
        assert r.json()["output_dir"] == "/tmp/entropy-test-2"


class TestRegressionSmoke:
    def test_config_still_works(self, client):
        r = client.get(f"{API}/config", timeout=15)
        assert r.status_code == 200
        for k in ("formats", "bitrates", "engines"):
            assert k in r.json()

    def test_clean_url_still_strips(self, client):
        url = "https://www.youtube.com/watch?v=dQw4w9WgXcQ&si=xx&utm_source=test"
        r = client.post(f"{API}/clean-url", json={"url": url}, timeout=15)
        assert r.status_code == 200
        cleaned = r.json()["url"]
        assert "si=" not in cleaned and "utm_" not in cleaned

    def test_search_youtube_still_works(self, client):
        r = client.post(f"{API}/search", json={"source": "youtube", "query": "lofi", "limit": 2}, timeout=60)
        assert r.status_code == 200
        assert len(r.json()["results"]) > 0

    def test_search_soundcloud_still_works(self, client):
        r = client.post(f"{API}/search", json={"source": "soundcloud", "query": "chill", "limit": 2}, timeout=60)
        assert r.status_code == 200
        assert len(r.json()["results"]) > 0

    def test_jobs_clear_still_works(self, client):
        r = client.post(f"{API}/jobs/clear", json={"what": "completed"}, timeout=15)
        assert r.status_code == 200
        assert "removed" in r.json()

    def test_sse_stream_still_works(self, client):
        r = requests.get(f"{API}/jobs/stream", stream=True, timeout=10)
        try:
            assert r.status_code == 200
            assert "text/event-stream" in r.headers.get("Content-Type", "")
            got_event = False
            start = time.time()
            for raw in r.iter_lines(decode_unicode=True):
                if time.time() - start > 6:
                    break
                if raw and raw.startswith("event:") and "snapshot" in raw:
                    got_event = True
                    break
            assert got_event
        finally:
            r.close()
