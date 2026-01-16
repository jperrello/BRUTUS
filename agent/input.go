package agent

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

var commands = []string{
	"/models",
	"/help",
	"/clear",
	"/exit",
}

type inputReader struct{}

func newInputReader() *inputReader {
	return &inputReader{}
}

func (r *inputReader) ReadLine(prompt string) (string, bool) {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return r.readLineSimple(prompt)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	fmt.Print(prompt)

	var line []byte
	var lastSuggestion string

	for {
		buf := make([]byte, 3)
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return "", false
		}

		if n == 1 {
			ch := buf[0]

			switch ch {
			case 3: // Ctrl+C
				fmt.Println("^C")
				return "", false

			case 4: // Ctrl+D
				if len(line) == 0 {
					fmt.Println()
					return "", false
				}

			case 13, 10: // Enter
				r.clearGhost(lastSuggestion, string(line))
				fmt.Println()
				return string(line), true

			case 127, 8: // Backspace
				if len(line) > 0 {
					r.clearGhost(lastSuggestion, string(line))
					line = line[:len(line)-1]
					fmt.Print("\b \b")
					lastSuggestion = r.updateGhost(string(line))
				}

			case 9: // Tab - accept suggestion
				if lastSuggestion != "" {
					r.clearGhost(lastSuggestion, string(line))
					rest := lastSuggestion[len(line):]
					line = []byte(lastSuggestion)
					fmt.Print(rest)
					lastSuggestion = ""
				}

			default:
				if ch >= 32 && ch < 127 {
					r.clearGhost(lastSuggestion, string(line))
					line = append(line, ch)
					fmt.Print(string(ch))
					lastSuggestion = r.updateGhost(string(line))
				}
			}
		} else if n == 3 && buf[0] == 27 && buf[1] == 91 {
			switch buf[2] {
			case 67: // Right arrow - accept one char
				if lastSuggestion != "" && len(lastSuggestion) > len(line) {
					r.clearGhost(lastSuggestion, string(line))
					ch := lastSuggestion[len(line)]
					line = append(line, ch)
					fmt.Print(string(ch))
					lastSuggestion = r.updateGhost(string(line))
				}
			}
		}
	}
}

func (r *inputReader) updateGhost(input string) string {
	suggestion := r.getSuggestion(input)
	if suggestion != "" && len(suggestion) > len(input) {
		ghost := suggestion[len(input):]
		fmt.Print("\033[90m" + ghost + "\033[0m")
		// Move cursor back to end of actual input
		for range ghost {
			fmt.Print("\b")
		}
	}
	return suggestion
}

func (r *inputReader) clearGhost(suggestion, current string) {
	if suggestion == "" || len(suggestion) <= len(current) {
		return
	}
	n := len(suggestion) - len(current)
	// Move forward, overwrite with spaces, move back
	for i := 0; i < n; i++ {
		fmt.Print(" ")
	}
	for i := 0; i < n; i++ {
		fmt.Print("\b")
	}
}

func (r *inputReader) getSuggestion(input string) string {
	if !strings.HasPrefix(input, "/") || input == "" {
		return ""
	}
	lower := strings.ToLower(input)
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, lower) && cmd != input {
			return cmd
		}
	}
	return ""
}

func (r *inputReader) readLineSimple(prompt string) (string, bool) {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return scanner.Text(), true
	}
	return "", false
}
