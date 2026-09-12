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
