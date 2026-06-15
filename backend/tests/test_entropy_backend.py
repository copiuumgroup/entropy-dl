"""Backend API tests for entropy-gui (Go HTTP service)."""
import os
import time
import pytest
import requests

BASE_URL = os.environ.get("REACT_APP_BACKEND_URL", "https://e10f40b8-5ce6-4479-b4c6-6ca6e6e3e95e.preview.emergentagent.com").rstrip("/")
API = f"{BASE_URL}/api"


@pytest.fixture(scope="module")
def client():
    s = requests.Session()
    s.headers.update({"Content-Type": "application/json"})
    return s


# ---------- Health & Config ----------
class TestHealth:
    def test_health(self, client):
        r = client.get(f"{API}/health", timeout=15)
        assert r.status_code == 200
        data = r.json()
        assert data["status"] == "ok"
        assert "service" in data

    def test_config(self, client):
        r = client.get(f"{API}/config", timeout=15)
        assert r.status_code == 200
        data = r.json()
        for k in ("formats", "bitrates", "engines"):
            assert k in data and isinstance(data[k], list) and len(data[k]) > 0
        assert "mp3" in data["formats"]
        assert "192" in data["bitrates"]
        assert "ytdlp" in data["engines"]


# ---------- Clean URL ----------
class TestCleanURL:
    def test_clean_youtube_with_tracking(self, client):
        url = "https://www.youtube.com/watch?v=dQw4w9WgXcQ&si=xx&pp=yy&utm_source=test&feature=share"
        r = client.post(f"{API}/clean-url", json={"url": url}, timeout=15)
        assert r.status_code == 200
        cleaned = r.json()["url"]
        for bad in ("si=", "pp=", "utm_", "feature="):
            assert bad not in cleaned, f"Tracking param {bad} still present: {cleaned}"
        assert "v=dQw4w9WgXcQ" in cleaned

    def test_clean_youtu_be_expansion(self, client):
        url = "https://youtu.be/dQw4w9WgXcQ?si=trackingvalue"
        r = client.post(f"{API}/clean-url", json={"url": url}, timeout=15)
        assert r.status_code == 200
        cleaned = r.json()["url"]
        # Expectation per spec: youtu.be/<id>?si=x -> www.youtube.com/watch?v=<id>
        assert "youtube.com/watch" in cleaned
        assert "v=dQw4w9WgXcQ" in cleaned
        assert "si=" not in cleaned

    def test_clean_text_multiple_urls(self, client):
        text = (
            "https://youtu.be/abcdEFG1234?si=foo\n"
            "https://www.youtube.com/watch?v=xyz&utm_source=test\n"
            "not-a-url-line\n"
            "https://soundcloud.com/artist/track?fbclid=zzz"
        )
        r = client.post(f"{API}/clean-url", json={"text": text}, timeout=15)
        assert r.status_code == 200
        urls = r.json()["urls"]
        assert isinstance(urls, list)
        assert len(urls) >= 3
        joined = "\n".join(urls)
        for bad in ("si=", "utm_", "fbclid="):
            assert bad not in joined


# ---------- Search ----------
class TestSearch:
    def test_search_youtube(self, client):
        r = client.post(f"{API}/search", json={"source": "youtube", "query": "lofi hip hop", "limit": 3}, timeout=60)
        assert r.status_code == 200, r.text
        results = r.json()["results"]
        assert isinstance(results, list)
        assert len(results) > 0
        first = results[0]
        for k in ("id", "title", "url", "source"):
            assert k in first
        assert first["source"].lower() in ("youtube", "yt")

    def test_search_soundcloud(self, client):
        r = client.post(f"{API}/search", json={"source": "soundcloud", "query": "chill", "limit": 2}, timeout=60)
        assert r.status_code == 200, r.text
        results = r.json()["results"]
        assert isinstance(results, list)
        assert len(results) > 0
        assert "soundcloud" in results[0].get("source", "").lower() or "soundcloud" in results[0].get("url", "").lower()

    def test_search_empty_query(self, client):
        r = client.post(f"{API}/search", json={"source": "youtube", "query": "", "limit": 3}, timeout=15)
        assert r.status_code == 400


# ---------- Jobs ----------
class TestJobs:
    def test_create_job_with_items(self, client):
        payload = {
            "items": [{
                "url": "https://www.youtube.com/watch?v=jNQXAC9IVRw",
                "title": "Me at the zoo"
            }],
            "options": {
                "format": "mp3",
                "bitrate": "192",
                "embed_meta": True,
                "embed_thumb": True,
                "engine": "ytdlp"
            }
        }
        r = client.post(f"{API}/jobs", json=payload, timeout=30)
        assert r.status_code in (200, 201), r.text
        jobs = r.json()["jobs"]
        assert isinstance(jobs, list) and len(jobs) == 1
        job = jobs[0]
        assert "id" in job
        assert job["status"] in ("queued", "downloading", "processing")
        TestJobs.job_id = job["id"]

    def test_list_jobs(self, client):
        r = client.get(f"{API}/jobs", timeout=15)
        assert r.status_code == 200
        jobs = r.json()["jobs"]
        assert isinstance(jobs, list)
        ids = [j["id"] for j in jobs]
        assert TestJobs.job_id in ids

    def test_no_items_no_urls(self, client):
        r = client.post(f"{API}/jobs", json={"options": {"format": "mp3"}}, timeout=15)
        assert r.status_code == 400

    def test_delete_job(self, client):
        # create a throwaway job
        r = client.post(f"{API}/jobs", json={
            "items": [{"url": "https://www.youtube.com/watch?v=oHg5SJYRHA0", "title": "TEST_to_delete"}],
            "options": {"format": "mp3", "engine": "ytdlp"}
        }, timeout=30)
        assert r.status_code in (200, 201)
        jid = r.json()["jobs"][0]["id"]

        r = client.delete(f"{API}/jobs/{jid}", timeout=15)
        assert r.status_code == 200
        assert r.json().get("ok") is True

        # Verify gone
        r = client.get(f"{API}/jobs", timeout=15)
        ids = [j["id"] for j in r.json()["jobs"]]
        assert jid not in ids

    def test_clear_completed(self, client):
        r = client.post(f"{API}/jobs/clear", json={"what": "completed"}, timeout=15)
        assert r.status_code == 200
        assert "removed" in r.json()
        assert isinstance(r.json()["removed"], int)


# ---------- SSE Stream ----------
class TestSSE:
    def test_stream_initial_snapshot(self, client):
        r = requests.get(f"{API}/jobs/stream", stream=True, timeout=10)
        try:
            assert r.status_code == 200
            ctype = r.headers.get("Content-Type", "")
            assert "text/event-stream" in ctype, f"Got content-type: {ctype}"

            got_event = False
            got_data = False
            start = time.time()
            for raw in r.iter_lines(decode_unicode=True):
                if time.time() - start > 8:
                    break
                if raw is None:
                    continue
                if raw.startswith("event:"):
                    got_event = True
                    assert "snapshot" in raw
                if raw.startswith("data:"):
                    got_data = True
                if got_event and got_data:
                    break
            assert got_event, "No event: line received from SSE"
            assert got_data, "No data: line received from SSE"
        finally:
            r.close()


# ---------- Progress (lightweight check) ----------
class TestProgress:
    def test_job_transitions_to_downloading(self, client):
        payload = {
            "items": [{
                "url": "https://www.youtube.com/watch?v=jNQXAC9IVRw",
                "title": "TEST_progress_check"
            }],
            "options": {"format": "mp3", "bitrate": "192", "engine": "ytdlp",
                        "embed_meta": False, "embed_thumb": False}
        }
        r = client.post(f"{API}/jobs", json=payload, timeout=30)
        assert r.status_code in (200, 201)
        jid = r.json()["jobs"][0]["id"]

        progressed = False
        deadline = time.time() + 25
        while time.time() < deadline:
            lr = client.get(f"{API}/jobs", timeout=15)
            jobs = lr.json()["jobs"]
            this = next((j for j in jobs if j["id"] == jid), None)
            if this and this["status"] in ("downloading", "processing", "done"):
                progressed = True
                break
            if this and this["status"] == "failed":
                pytest.skip(f"Download failed (likely network/yt-dlp issue): {this.get('error')}")
            time.sleep(2)
        assert progressed, "Job did not transition out of queued state within 25s"

        # cleanup
        client.delete(f"{API}/jobs/{jid}", timeout=15)
