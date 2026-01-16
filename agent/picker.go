package agent

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

func pickFromList(title string, items []string, pageSize int) (int, error) {
	if len(items) == 0 {
		return -1, fmt.Errorf("no items to pick from")
	}

	if pageSize <= 0 {
		pageSize = 10
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		// Not a terminal - show list and return first item
		fmt.Printf("%s\n", title)
		for i, item := range items {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(items)-10)
				break
			}
			fmt.Printf("  %s\n", item)
		}
		return 0, fmt.Errorf("interactive selection requires a terminal")
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	selected := 0
	offset := 0

	for {
		// Clear screen and draw
		fmt.Print("\033[2J\033[H")
		fmt.Printf("\033[1;36m%s\033[0m\n", title)
		fmt.Println("\033[90mUse ↑/↓ to navigate, Enter to select, q to cancel\033[0m")
		fmt.Println()

		// Calculate visible range
		end := offset + pageSize
		if end > len(items) {
			end = len(items)
		}

		for i := offset; i < end; i++ {
			if i == selected {
				fmt.Printf("\033[1;33m> %s\033[0m\n", items[i])
			} else {
				fmt.Printf("  %s\n", items[i])
			}
		}

		// Show scroll indicators
		fmt.Println()
		if len(items) > pageSize {
			fmt.Printf("\033[90m[%d/%d]", selected+1, len(items))
			if offset > 0 {
				fmt.Print(" ↑ more above")
			}
			if end < len(items) {
				fmt.Print(" ↓ more below")
			}
			fmt.Println("\033[0m")
		}

		// Read input
		buf := make([]byte, 3)
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return -1, err
		}

		if n == 1 {
			switch buf[0] {
			case 'q', 'Q', 27: // q, Q, or Escape
				fmt.Print("\033[2J\033[H")
				return -1, nil
			case 13, 10: // Enter
				fmt.Print("\033[2J\033[H")
				return selected, nil
			case 'j', 'J': // vim down
				if selected < len(items)-1 {
					selected++
					if selected >= offset+pageSize {
						offset++
					}
				}
			case 'k', 'K': // vim up
				if selected > 0 {
					selected--
					if selected < offset {
						offset--
					}
				}
			}
		} else if n == 3 && buf[0] == 27 && buf[1] == 91 {
			switch buf[2] {
			case 65: // Up arrow
				if selected > 0 {
					selected--
					if selected < offset {
						offset--
					}
				}
			case 66: // Down arrow
				if selected < len(items)-1 {
					selected++
					if selected >= offset+pageSize {
						offset++
					}
				}
			}
		}
	}
}

func filterItems(items []string, query string) []int {
	query = strings.ToLower(query)
	var matches []int
	for i, item := range items {
		if strings.Contains(strings.ToLower(item), query) {
			matches = append(matches, i)
		}
	}
	return matches
}
