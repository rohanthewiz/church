package formdraft

import "testing"

func TestSameItem(t *testing.T) {
	cases := []struct {
		draftID string
		items   []int64
		want    bool
	}{
		{"", nil, true},
		{"0", nil, true},
		{"7", nil, false},
		{"7", []int64{7}, true},
		{" 7 ", []int64{7}, true},
		{"8", []int64{7}, false},
		{"", []int64{7}, false},
	}
	for _, c := range cases {
		if got := SameItem(c.draftID, c.items); got != c.want {
			t.Errorf("SameItem(%q, %v) = %v, want %v", c.draftID, c.items, got, c.want)
		}
	}
}

func TestKeyIgnoresQuery(t *testing.T) {
	if key("s1", "/admin/articles/edit/7?x=1") != key("s1", "/admin/articles/edit/7") {
		t.Error("a query string changed the draft key")
	}
	if key("s1", "/admin/articles/new") == key("s2", "/admin/articles/new") {
		t.Error("two sessions share a draft key")
	}
}

func TestFromParams(t *testing.T) {
	var v struct{ Title string }
	if FromParams(map[string]map[string]string{}, &v) {
		t.Error("no draft reported as present")
	}
	if FromParams(map[string]map[string]string{"_global": {ParamKey: "{not json"}}, &v) {
		t.Error("undecodable draft reported as present")
	}
	if !FromParams(map[string]map[string]string{"_global": {ParamKey: `{"Title":"typed"}`}}, &v) || v.Title != "typed" {
		t.Errorf("draft not decoded: %+v", v)
	}
}
