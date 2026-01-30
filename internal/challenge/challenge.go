package challenge

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

const statement = `I UNDERSTAND THIS ACTION WILL DECREASE,
AND POTENTIALLY ELIMINATE, MY PRODUCTIVITY.
I UNDERSTAND THE RISK INVOLVED,
AND I AM WILLING TO PROCEED.`

const maxRetriesPerLine = 3

// Require runs the typing challenge and wraps any error with the given context.
// Use this as a standardized way to require a challenge before dangerous operations.
func Require(context string) error {
	if err := Run(); err != nil {
		return fmt.Errorf("%s: %w", context, err)
	}
	return nil
}

// Run executes the typing challenge
func Run() error {
	lines := strings.Split(statement, "\n")
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n⚠️  WARNING: You are about to perform an action that may reduce your productivity.")
	fmt.Println("To proceed, you must type the following statement line by line.")

	for i, line := range lines {
		retries := 0
		for {
			// Display the line with typewriter effect
			fmt.Print("\nType this line: ")
			typewriterEffect(line)
			fmt.Println()

			// Prompt for input
			fmt.Print("> ")
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}

			// Trim only the newline character, preserve spaces
			input = strings.TrimSuffix(input, "\n")
			input = strings.TrimSuffix(input, "\r") // Handle Windows line endings

			// Check if input matches exactly
			if input == line {
				if i < len(lines)-1 {
					fmt.Println("✓ Correct. Continue to the next line.")
				}
				break
			}

			retries++
			if retries >= maxRetriesPerLine {
				return fmt.Errorf("too many incorrect attempts. Challenge failed")
			}

			fmt.Printf("✗ Incorrect. You have %d attempt(s) remaining for this line.\n", maxRetriesPerLine-retries)
		}
	}

	fmt.Println("\n✓ Challenge completed successfully.")
	return nil
}

// typewriterEffect prints text character by character with a delay
func typewriterEffect(text string) {
	for _, char := range text {
		fmt.Print(string(char))
		// Random delay between 30-50ms per character
		delay := time.Duration(30+rand.Intn(21)) * time.Millisecond
		time.Sleep(delay)
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
