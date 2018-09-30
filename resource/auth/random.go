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
		if count < 50 { fmt.Printf("'%s',", trimedLine) }
		randStrings = append(randStrings, trimedLine)
	}
	fmt.Printf(" ...\n%d seeds read from random seeds file\n", count)
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
