package jobs

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// helper: create a minimal Manager for unit-testing owner scoping
// (no real yt-dlp/ffmpeg/aria2c needed since we don't run jobs).
func newTestManager() *Manager {
	return NewManager("yt-dlp-test", "aria2c-test", "ffmpeg-test", "test-audio", "test-video", 1)
}

// addTestJob creates a job directly in the manager's internal maps
// without triggering yt-dlp probing or enqueueing.
func (m *Manager) addTestJob(url, title, owner string, status Status) *Job {
	j := &Job{
		ID:        newUUID(),
		URL:       url,
		Title:     title,
		Status:    status,
		Stage:     string(status),
		Owner:     owner,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.mu.Lock()
	m.jobs[j.ID] = j
	m.order = append(m.order, j.ID)
	m.mu.Unlock()
	return j
}

var testIDCounter atomic.Int64

func newUUID() string {
	n := testIDCounter.Add(1)
	return fmt.Sprintf("test-%d", n)
}

// --- List ---

func TestList_AdminSeesAll(t *testing.T) {
	m := newTestManager()
	m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)
	m.addTestJob("https://b.com/1", "B1", "bob", StatusFailed)
	m.addTestJob("https://c.com/1", "Legacy", "", StatusDone) // legacy

	jobs := m.List("") // admin/loopback sees all
	if len(jobs) != 3 {
		t.Fatalf("admin should see 3 jobs, got %d", len(jobs))
	}
}

func TestList_UserSeesOwnAndLegacy(t *testing.T) {
	m := newTestManager()
	m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)
	m.addTestJob("https://b.com/1", "B1", "bob", StatusFailed)
	m.addTestJob("https://c.com/1", "Legacy", "", StatusDone)

	jobs := m.List("alice")
	if len(jobs) != 2 {
		t.Fatalf("alice should see 2 jobs (own + legacy), got %d", len(jobs))
	}
	for _, j := range jobs {
		if j.Owner != "alice" && j.Owner != "" {
			t.Errorf("alice saw bob's job: %+v", j)
		}
	}

	jobs = m.List("bob")
	if len(jobs) != 2 {
		t.Fatalf("bob should see 2 jobs (own + legacy), got %d", len(jobs))
	}

	jobs = m.List("carol") // no jobs, only legacy
	if len(jobs) != 1 {
		t.Fatalf("carol should see 1 legacy job, got %d", len(jobs))
	}
}

func TestList_EmptyManager(t *testing.T) {
	m := newTestManager()
	if len(m.List("")) != 0 {
		t.Fatal("admin on empty manager should see 0 jobs")
	}
	if len(m.List("alice")) != 0 {
		t.Fatal("user on empty manager should see 0 jobs")
	}
}

// --- Get ---

func TestGet_AdminSeesAny(t *testing.T) {
	m := newTestManager()
	j := m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)

	got, ok := m.Get(j.ID, "")
	if !ok || got.Owner != "alice" {
		t.Fatal("admin should get alice's job")
	}
}

func TestGet_OwnerCanGetOwn(t *testing.T) {
	m := newTestManager()
	j := m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)

	got, ok := m.Get(j.ID, "alice")
	if !ok {
		t.Fatal("alice should get her own job")
	}
	if got.URL != j.URL {
		t.Fatalf("wrong job: got %q, want %q", got.URL, j.URL)
	}
}

func TestGet_UserCannotGetOthers(t *testing.T) {
	m := newTestManager()
	j := m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)

	_, ok := m.Get(j.ID, "bob")
	if ok {
		t.Fatal("bob should NOT see alice's job")
	}
}

func TestGet_UserCanGetLegacy(t *testing.T) {
	m := newTestManager()
	j := m.addTestJob("https://c.com/1", "Legacy", "", StatusDone)

	_, ok := m.Get(j.ID, "alice")
	if !ok {
		t.Fatal("alice should be able to get legacy (Owner=\"\") job")
	}
}

func TestGet_MissingJob(t *testing.T) {
	m := newTestManager()
	_, ok := m.Get("nonexistent", "")
	if ok {
		t.Fatal("should return false for missing job")
	}
}

// --- Retry ---

func TestRetry_AdminCanRetryAny(t *testing.T) {
	m := newTestManager()
	j := m.addTestJob("https://a.com/1", "A1", "alice", StatusFailed)

	if !m.Retry(j.ID, "") {
		t.Fatal("admin should retry alice's failed job")
	}
	got, _ := m.Get(j.ID, "")
	if got.Status != StatusQueued {
		t.Fatalf("status should be queued, got %q", got.Status)
	}
}

func TestRetry_OwnerCanRetryOwn(t *testing.T) {
	m := newTestManager()
	j := m.addTestJob("https://a.com/1", "A1", "alice", StatusFailed)

	if !m.Retry(j.ID, "alice") {
		t.Fatal("alice should retry her own failed job")
	}
}

func TestRetry_UserCannotRetryOthers(t *testing.T) {
	m := newTestManager()
	j := m.addTestJob("https://a.com/1", "A1", "alice", StatusFailed)

	if m.Retry(j.ID, "bob") {
		t.Fatal("bob should NOT retry alice's job")
	}
}

func TestRetry_NonFailedCannotRetry(t *testing.T) {
	m := newTestManager()
	j := m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)

	if m.Retry(j.ID, "alice") {
		t.Fatal("should not retry a done job")
	}
}

func TestRetry_CanceledJob(t *testing.T) {
	m := newTestManager()
	j := m.addTestJob("https://a.com/1", "A1", "alice", StatusCanceled)

	if !m.Retry(j.ID, "") {
		t.Fatal("admin should retry a canceled job")
	}
}

// --- RetryAllFailed ---

func TestRetryAllFailed_Admin(t *testing.T) {
	m := newTestManager()
	m.addTestJob("https://a.com/1", "A1", "alice", StatusFailed)
	m.addTestJob("https://b.com/1", "B1", "bob", StatusFailed)
	m.addTestJob("https://c.com/1", "C1", "", StatusFailed) // legacy
	m.addTestJob("https://d.com/1", "D1", "alice", StatusDone)

	retried := m.RetryAllFailed("")
	if retried != 3 {
		t.Fatalf("admin should retry 3 failed jobs, got %d", retried)
	}
}

func TestRetryAllFailed_User(t *testing.T) {
	m := newTestManager()
	m.addTestJob("https://a.com/1", "A1", "alice", StatusFailed)
	m.addTestJob("https://b.com/1", "B1", "bob", StatusFailed)
	m.addTestJob("https://c.com/1", "C1", "", StatusFailed) // legacy

	retried := m.RetryAllFailed("alice")
	// alice sees her own (1) + legacy (1) = 2
	if retried != 2 {
		t.Fatalf("alice should retry 2 jobs (own + legacy), got %d", retried)
	}
}

// --- Remove ---

func TestRemove_AdminCanRemoveAny(t *testing.T) {
	m := newTestManager()
	j := m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)

	if !m.Remove(j.ID, "") {
		t.Fatal("admin should remove alice's job")
	}
	if len(m.List("")) != 0 {
		t.Fatal("should be empty after removal")
	}
}

func TestRemove_OwnerCanRemoveOwn(t *testing.T) {
	m := newTestManager()
	j := m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)

	if !m.Remove(j.ID, "alice") {
		t.Fatal("alice should remove her own job")
	}
}

func TestRemove_UserCannotRemoveOthers(t *testing.T) {
	m := newTestManager()
	j := m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)

	if m.Remove(j.ID, "bob") {
		t.Fatal("bob should NOT remove alice's job")
	}
	if len(m.List("alice")) != 1 {
		t.Fatal("alice's job should still exist")
	}
}

func TestRemove_MissingJob(t *testing.T) {
	m := newTestManager()
	if m.Remove("nonexistent", "") {
		t.Fatal("should return false for missing job")
	}
}

// --- Clear ---

func TestClear_Admin(t *testing.T) {
	m := newTestManager()
	m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)
	m.addTestJob("https://b.com/1", "B1", "bob", StatusDone)
	m.addTestJob("https://c.com/1", "C1", "", StatusFailed)

	removed := m.Clear("completed", "")
	if removed != 2 {
		t.Fatalf("admin should clear 2 completed jobs, got %d", removed)
	}
	if len(m.List("")) != 1 {
		t.Fatal("only the failed job should remain")
	}
}

func TestClear_User(t *testing.T) {
	m := newTestManager()
	m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)
	m.addTestJob("https://a.com/2", "A2", "alice", StatusFailed)
	m.addTestJob("https://b.com/1", "B1", "bob", StatusDone)
	m.addTestJob("https://c.com/1", "C1", "", StatusDone) // legacy

	removed := m.Clear("completed", "alice")
	// alice sees her own (1 done) + legacy (1 done) = 2 completed
	if removed != 2 {
		t.Fatalf("alice should clear 2 completed (own + legacy), got %d", removed)
	}
	// bob's done job should remain
	if len(m.List("")) != 2 { // bob's done + alice's failed
		t.Fatalf("expected 2 remaining jobs, got %d", len(m.List("")))
	}
}

func TestClear_Failed(t *testing.T) {
	m := newTestManager()
	m.addTestJob("https://a.com/1", "A1", "alice", StatusFailed)
	m.addTestJob("https://a.com/2", "A2", "alice", StatusDone)

	removed := m.Clear("failed", "")
	if removed != 1 {
		t.Fatalf("should clear 1 failed, got %d", removed)
	}
}

// --- IsDuplicateURL ---

func TestIsDuplicateURL_Admin(t *testing.T) {
	m := newTestManager()
	m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)
	m.addTestJob("https://b.com/1", "B1", "bob", StatusFailed) // not done

	if !m.IsDuplicateURL("https://a.com/1", "") {
		t.Fatal("admin should see alice's done URL as duplicate")
	}
	if m.IsDuplicateURL("https://b.com/1", "") {
		t.Fatal("bob's failed URL is not a duplicate (not done)")
	}
}

func TestIsDuplicateURL_User(t *testing.T) {
	m := newTestManager()
	m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)
	m.addTestJob("https://b.com/1", "B1", "bob", StatusDone)

	// alice can see her own done URL
	if !m.IsDuplicateURL("https://a.com/1", "alice") {
		t.Fatal("alice should see her own done URL as duplicate")
	}
	// alice cannot see bob's done URL
	if m.IsDuplicateURL("https://b.com/1", "alice") {
		t.Fatal("alice should NOT see bob's done URL as duplicate")
	}
}

// --- DuplicateCount ---

func TestDuplicateCount_Admin(t *testing.T) {
	m := newTestManager()
	m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)
	m.addTestJob("https://b.com/1", "B1", "bob", StatusDone)
	m.addTestJob("https://c.com/1", "C1", "carol", StatusFailed)

	count := m.DuplicateCount([]string{"https://a.com/1", "https://b.com/1", "https://c.com/1"}, "")
	if count != 2 {
		t.Fatalf("admin should count 2 duplicates, got %d", count)
	}
}

func TestDuplicateCount_User(t *testing.T) {
	m := newTestManager()
	m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)
	m.addTestJob("https://b.com/1", "B1", "bob", StatusDone)

	count := m.DuplicateCount([]string{"https://a.com/1", "https://b.com/1"}, "alice")
	if count != 1 {
		t.Fatalf("alice should count 1 duplicate (her own), got %d", count)
	}
}

// --- Stats ---

func TestStats_Admin(t *testing.T) {
	m := newTestManager()
	m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)
	m.addTestJob("https://b.com/1", "B1", "bob", StatusFailed)
	m.addTestJob("https://c.com/1", "C1", "", StatusQueued)

	s := m.Stats("")
	if s.Total != 3 {
		t.Fatalf("admin total should be 3, got %d", s.Total)
	}
	if s.Done != 1 || s.Failed != 1 || s.Queued != 1 {
		t.Fatalf("wrong breakdown: %+v", s)
	}
}

func TestStats_User(t *testing.T) {
	m := newTestManager()
	m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)
	m.addTestJob("https://a.com/2", "A2", "alice", StatusFailed)
	m.addTestJob("https://b.com/1", "B1", "bob", StatusDone)
	m.addTestJob("https://c.com/1", "C1", "", StatusQueued) // legacy

	s := m.Stats("alice")
	// alice: own done (1) + own failed (1) + legacy queued (1) = 3
	if s.Total != 3 {
		t.Fatalf("alice total should be 3, got %d", s.Total)
	}
	if s.Done != 1 || s.Failed != 1 || s.Queued != 1 {
		t.Fatalf("wrong breakdown for alice: %+v", s)
	}
}

func TestStats_Empty(t *testing.T) {
	m := newTestManager()
	s := m.Stats("")
	if s.Total != 0 {
		t.Fatalf("empty stats should have 0 total, got %d", s.Total)
	}
}

// --- RecentLogs ---

func TestRecentLogs_OwnerFiltering(t *testing.T) {
	m := newTestManager()
	ja := m.addTestJob("https://a.com/1", "A1", "alice", StatusDownloading)
	jb := m.addTestJob("https://b.com/1", "B1", "bob", StatusDownloading)

	m.appendLog(ja.ID, "alice log line 1")
	m.appendLog(jb.ID, "bob log line 1")
	m.appendLog(ja.ID, "alice log line 2")

	logs := m.RecentLogs(100, "alice")
	if len(logs) != 2 {
		t.Fatalf("alice should see 2 log lines, got %d", len(logs))
	}
	for _, ll := range logs {
		if ll.Owner != "alice" && ll.Owner != "" {
			t.Errorf("alice saw bob's log: %+v", ll)
		}
	}

	logs = m.RecentLogs(100, "bob")
	if len(logs) != 1 {
		t.Fatalf("bob should see 1 log line, got %d", len(logs))
	}
}

func TestRecentLogs_AdminSeesAll(t *testing.T) {
	m := newTestManager()
	ja := m.addTestJob("https://a.com/1", "A1", "alice", StatusDownloading)
	m.appendLog(ja.ID, "alice log line")

	logs := m.RecentLogs(100, "")
	if len(logs) != 1 {
		t.Fatalf("admin should see 1 log line, got %d", len(logs))
	}
}

// --- Subscribe / broadcast ---

func TestSubscribe_AdminSeesAllEvents(t *testing.T) {
	m := newTestManager()
	subID, ch, err := m.Subscribe("")
	if err != nil {
		t.Fatal(err)
	}
	defer m.Unsubscribe(subID)

	ja := m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)
	jb := m.addTestJob("https://b.com/1", "B1", "bob", StatusDone)

	// updateJob triggers broadcast
	m.mu.RLock()
	m.updateJob(ja)
	m.updateJob(jb)
	m.mu.RUnlock()

	// Drain events
	timeout := time.After(200 * time.Millisecond)
	var count int
loop:
	for {
		select {
		case <-ch:
			count++
		case <-timeout:
			break loop
		}
	}
	// Should have received both job update events
	if count < 2 {
		t.Fatalf("admin subscriber should get events for both jobs, got %d", count)
	}
}

func TestSubscribe_UserSeesOwnEvents(t *testing.T) {
	m := newTestManager()
	subID, ch, err := m.Subscribe("alice")
	if err != nil {
		t.Fatal(err)
	}
	defer m.Unsubscribe(subID)

	ja := m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)
	jb := m.addTestJob("https://b.com/1", "B1", "bob", StatusDone)

	m.mu.RLock()
	m.updateJob(ja)
	m.updateJob(jb)
	m.mu.RUnlock()

	timeout := time.After(200 * time.Millisecond)
	var received []Event
loop:
	for {
		select {
		case e := <-ch:
			received = append(received, e)
		case <-timeout:
			break loop
		}
	}
	for _, e := range received {
		if e.Job != nil && e.Job.Owner == "bob" {
			t.Fatal("alice subscriber should NOT receive bob's job event")
		}
	}
}

func TestBroadcastSnapshot_PerSubscriber(t *testing.T) {
	m := newTestManager()
	m.addTestJob("https://a.com/1", "A1", "alice", StatusDone)
	m.addTestJob("https://b.com/1", "B1", "bob", StatusDone)
	m.addTestJob("https://c.com/1", "C1", "", StatusDone) // legacy

	adminSub, adminCh, _ := m.Subscribe("")
	aliceSub, aliceCh, _ := m.Subscribe("alice")
	bobSub, bobCh, _ := m.Subscribe("bob")
	defer m.Unsubscribe(adminSub)
	defer m.Unsubscribe(aliceSub)
	defer m.Unsubscribe(bobSub)

	m.broadcastSnapshot()

	// Read from all 3 channels concurrently to avoid sequential timeouts.
	type snapResult struct {
		label string
		event Event
	}
	ch := make(chan snapResult, 3)
	read := func(label string, c chan Event) {
		for {
			select {
			case e := <-c:
				if e.Type == "snapshot" {
					ch <- snapResult{label, e}
					return
				}
			case <-time.After(200 * time.Millisecond):
				ch <- snapResult{label: label}
				return
			}
		}
	}
	go read("admin", adminCh)
	go read("alice", aliceCh)
	go read("bob", bobCh)

	for i := 0; i < 3; i++ {
		r := <-ch
		switch r.label {
		case "admin":
			if len(r.event.Jobs) != 3 {
				t.Errorf("admin snapshot should have 3 jobs, got %d", len(r.event.Jobs))
			}
		case "alice":
			if len(r.event.Jobs) != 2 {
				t.Errorf("alice snapshot should have 2 jobs (own + legacy), got %d", len(r.event.Jobs))
			}
		case "bob":
			if len(r.event.Jobs) != 2 {
				t.Errorf("bob snapshot should have 2 jobs (own + legacy), got %d", len(r.event.Jobs))
			}
		}
	}
}

// --- filterByOwner ---

func TestFilterByOwner(t *testing.T) {
	jobs := []*Job{
		{ID: "1", Owner: "alice"},
		{ID: "2", Owner: "bob"},
		{ID: "3", Owner: ""}, // legacy
	}

	out := filterByOwner(jobs, "alice")
	if len(out) != 2 {
		t.Fatalf("alice should see 2 (own + legacy), got %d", len(out))
	}

	out = filterByOwner(jobs, "carol") // no owned, only legacy
	if len(out) != 1 {
		t.Fatalf("carol should see 1 (legacy), got %d", len(out))
	}
}

// --- eventMatchesOwner ---

func TestEventMatchesOwner(t *testing.T) {
	jobEvent := Event{Type: "job", Job: &Job{Owner: "alice"}}
	logEvent := Event{Type: "log", Log: &LogLine{Owner: "bob"}}

	// Admin sees everything
	if !eventMatchesOwner(jobEvent, "") {
		t.Fatal("admin should see job event")
	}
	if !eventMatchesOwner(logEvent, "") {
		t.Fatal("admin should see log event")
	}

	// Alice sees her job but not bob's log
	if !eventMatchesOwner(jobEvent, "alice") {
		t.Fatal("alice should see her own job event")
	}
	if eventMatchesOwner(logEvent, "alice") {
		t.Fatal("alice should NOT see bob's log event")
	}

	// Bob sees his log but not alice's job
	if eventMatchesOwner(jobEvent, "bob") {
		t.Fatal("bob should NOT see alice's job event")
	}
	if !eventMatchesOwner(logEvent, "bob") {
		t.Fatal("bob should see his own log event")
	}

	// Snapshot events are NOT matched by eventMatchesOwner
	snapEvent := Event{Type: "snapshot"}
	if eventMatchesOwner(snapEvent, "alice") {
		t.Fatal("snapshot should return false for non-admin (handled by broadcastSnapshot)")
	}
	if !eventMatchesOwner(snapEvent, "") {
		t.Fatal("snapshot should return true for admin")
	}
}

// --- Subscribe limit ---

func TestSubscribe_MaxLimit(t *testing.T) {
	m := newTestManager()
	var subs []string
	for i := 0; i < maxSubscriptions; i++ {
		id, _, err := m.Subscribe("")
		if err != nil {
			t.Fatalf("sub %d failed: %v", i, err)
		}
		subs = append(subs, id)
	}

	_, _, err := m.Subscribe("")
	if err == nil {
		t.Fatal("should fail when exceeding max subscriptions")
	}

	// Clean up
	for _, id := range subs {
		m.Unsubscribe(id)
	}
}
