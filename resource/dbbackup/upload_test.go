package dbbackup

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"
)

// memStore is an in-memory replicate.Storage with List in ascending order,
// as the interface requires.
type memStore struct {
	objects map[string][]byte
	failPut string // key whose Put fails
}

func newMemStore() *memStore { return &memStore{objects: map[string][]byte{}} }

func (m *memStore) Put(_ context.Context, key string, data []byte) error {
	if key == m.failPut {
		return errors.New("put failed")
	}
	m.objects[key] = append([]byte(nil), data...)
	return nil
}

func (m *memStore) Get(_ context.Context, key string) ([]byte, error) {
	d, ok := m.objects[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return d, nil
}

func (m *memStore) List(_ context.Context, prefix string) ([]string, error) {
	var keys []string
	for k := range m.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func (m *memStore) Delete(_ context.Context, key string) error {
	delete(m.objects, key)
	return nil
}

func TestUploadWritesSnapshotAndLatest(t *testing.T) {
	st := newMemStore()
	now := time.Date(2026, 9, 17, 8, 5, 3, 0, time.FixedZone("CDT", -5*3600))

	res, err := upload(context.Background(), st, "cema", 0, []byte("db-bytes"), now)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	// The timestamp is UTC whatever the local zone
	if res.Key != "cema/20260917-130503Z/church.db" || res.LatestKey != "cema/latest/church.db" {
		t.Errorf("keys = %q, %q", res.Key, res.LatestKey)
	}
	if string(st.objects[res.Key]) != "db-bytes" || string(st.objects[res.LatestKey]) != "db-bytes" {
		t.Errorf("objects not written: %v", st.objects)
	}
	if res.Bytes != 8 {
		t.Errorf("Bytes = %d, want 8", res.Bytes)
	}
}

func TestUploadLeavesLatestAloneWhenSnapshotFails(t *testing.T) {
	st := newMemStore()
	st.objects["cema/latest/church.db"] = []byte("previous")
	now := time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC)
	st.failPut = "cema/20260917-130000Z/church.db"

	if _, err := upload(context.Background(), st, "cema", 0, []byte("new"), now); err == nil {
		t.Fatal("upload succeeded despite a failed snapshot put")
	}
	if string(st.objects["cema/latest/church.db"]) != "previous" {
		t.Error("latest/ moved past a snapshot that wasn't written")
	}
}

func TestPruneKeepsRetainAndIgnoresOtherKeys(t *testing.T) {
	st := newMemStore()
	base := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		st.objects["cema/"+base.Add(time.Duration(i)*time.Hour).Format(tsFormat)+"/church.db"] = []byte("x")
	}
	st.objects["cema/latest/church.db"] = []byte("x")
	st.objects["cema/wal/gen/0001/chunk"] = []byte("x")
	st.objects["cema/notes.txt"] = []byte("x")
	st.objects["cemaother/20260901-000000Z/church.db"] = []byte("x") // another site sharing a name prefix

	deleted, err := prune(context.Background(), st, "cema", 2)
	if err != nil || deleted != 3 {
		t.Fatalf("prune: deleted %d err %v, want 3", deleted, err)
	}
	for _, kept := range []string{
		"cema/20260901-030000Z/church.db", "cema/20260901-040000Z/church.db", // newest two
		"cema/latest/church.db", "cema/wal/gen/0001/chunk", "cema/notes.txt",
		"cemaother/20260901-000000Z/church.db",
	} {
		if _, ok := st.objects[kept]; !ok {
			t.Errorf("prune removed %s", kept)
		}
	}
	if _, ok := st.objects["cema/20260901-000000Z/church.db"]; ok {
		t.Error("prune kept the oldest snapshot")
	}
}
