package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"
	"os"
	"bufio"
	"strings"
		"log"
)

var randStrings []string

func init() {
	// Load crypto seeds
	seedsFile, err := os.Open("cfg/random_seeds.txt")
	if err != nil {
		log.Fatal("Error opening crypto seeds file")
	}
	scanner := bufio.NewScanner(seedsFile)

	var count int

	for scanner.Scan() { // splits on lines by default
		line := scanner.Text(); count++
		trimedLine := strings.TrimSpace(line)
		// The seeds are secret input to RandomKey (session keys, the superadmin
		// bootstrap token), so none of them are echoed. Printing even a prefix put
		// them in stdout on every boot, which in k8s means pod logs and whatever
		// log shipping sits behind them. The count alone is enough to confirm the
		// file loaded.
		randStrings = append(randStrings, trimedLine)
	}
	fmt.Printf("%d seeds read from random seeds file\n", count)
	if err := scanner.Err(); err != nil {
		log.Fatal("Error when reading random seeds file")
	}
	if err := checkSeedsForEnv(os.Getenv("APP_ENV"), randStrings); err != nil {
		log.Fatal(err.Error())
	}
}

// testSeedPrefix marks the dummy seeds committed as test fixtures
// (cfg/random_seeds.txt at the repo root and in each package's cfg/, which
// `go test` needs because this init runs in every test binary).
const testSeedPrefix = "test-seed-"

// checkSeedsForEnv refuses production when the seeds are the public test
// fixture or too few to be a real pool. The seeds feed session keys and the
// SuperAdmin bootstrap token (RandomKey), so a production site on the fixture
// would draw them from a list anyone with the source can read.
//
// APP_ENV is read directly rather than through config.AppEnv: this runs from
// init(), before main() has called config.InitConfig. Development and test
// runs are not checked, since that is exactly where the fixture belongs.
func checkSeedsForEnv(appEnv string, seeds []string) error {
	if strings.TrimSpace(appEnv) != "production" {
		return nil
	}
	usable := 0
	for _, s := range seeds {
		if s == "" || strings.HasPrefix(s, testSeedPrefix) {
			continue
		}
		usable++
	}
	// A generated pool has 60–72 lines (see deploy.sh cmd_seeds); 16 is a
	// floor well under that, above which a pool is plausibly deliberate.
	if usable < 16 {
		return fmt.Errorf("cfg/random_seeds.txt holds %d usable seeds (test fixtures and blank lines "+
			"don't count); production needs a generated pool of at least 16. "+
			"Run `./deploy/deploy.sh seeds` or generate one with openssl rand", usable)
	}
	return nil
}

// Randomly pull a string from the above array
func RandomString() string {
	return randStrings[RandomInt(int64(len(randStrings)-1))]
}

func RandomInt(max int64) int64 {
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		fmt.Println("Rand Int generator failed")
	}
	return int64(n.Int64())
}

func RandomKey() string {
	return PasswordHash(RandomString(),
		fmt.Sprintf("%s%d%s", RandomString(), time.Now().UnixNano(), RandomString()))
}
