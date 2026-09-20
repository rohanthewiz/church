package auth

import (
	"os"
	"strings"
	"testing"
)

// The sites' cfg/random_seeds.txt.sample files are committed, so whatever they
// contain is readable by everyone with the source. They once held the same real
// seeds cema ran on locally, which meant a `cp` of the sample produced a pool
// that looked generated and so sailed past checkSeedsForEnv.
//
// This feeds each committed sample through the very guard the boot path calls,
// pinning both halves of the contract: production refuses the sample, and local
// development still accepts it (copying it is the documented dev setup).
//
// Each site is a subtest because t.Skip unwinds the whole test function; a
// missing cema must not silently take ccswm's check with it.
func TestCommittedSamplesRefusedInProduction(t *testing.T) {
	samples := map[string]string{
		"cema":  "../../../cema/cfg/random_seeds.txt.sample",
		"ccswm": "../../../ccswm/cfg/random_seeds.txt.sample",
	}

	for site, path := range samples {
		t.Run(site, func(t *testing.T) {
			b, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				// The sites are separate repos. CI checks out church alone, so the
				// siblings are absent there; this check is for the workspace
				// checkout (go.work), where all three are present.
				t.Skipf("%s not present — church-only checkout", path)
			}
			if err != nil {
				t.Fatal(err)
			}

			var seeds []string
			for _, line := range strings.Split(string(b), "\n") {
				seeds = append(seeds, strings.TrimSpace(line))
			}

			if err := checkSeedsForEnv("production", seeds); err == nil {
				t.Error("production accepted the committed sample; it must hold placeholders only")
			}
			if err := checkSeedsForEnv("development", seeds); err != nil {
				t.Errorf("development refused the sample, but local dev must still boot: %v", err)
			}
		})
	}
}
