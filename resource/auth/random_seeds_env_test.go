package auth

import (
	"fmt"
	"testing"
)

func TestCheckSeedsForEnv(t *testing.T) {
	fixture := make([]string, 60)
	for i := range fixture {
		fixture[i] = fmt.Sprintf("test-seed-%02d", i+1)
	}
	generated := make([]string, 72)
	for i := range generated {
		generated[i] = fmt.Sprintf("q8Zr%015d", i)
	}

	cases := []struct {
		name    string
		env     string
		seeds   []string
		wantErr bool
	}{
		{"fixture in development", "development", fixture, false},
		{"fixture with APP_ENV unset", "", fixture, false},
		{"fixture in production", "production", fixture, true},
		{"too few in production", "production", generated[:5], true},
		{"blank lines don't count", "production", append(make([]string, 40), generated[:3]...), true},
		{"generated pool in production", " production ", generated, false},
	}
	for _, c := range cases {
		if err := checkSeedsForEnv(c.env, c.seeds); (err != nil) != c.wantErr {
			t.Errorf("%s: err = %v, wantErr %v", c.name, err, c.wantErr)
		}
	}
}
