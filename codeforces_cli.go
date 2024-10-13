package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/eiannone/keyboard"
	"github.com/schollz/closestmatch"
	"golang.org/x/term"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var tags = []string{
	"combine-tags-by-or", "2-sat", "binary search", "bitmasks", "brute force",
	"chinese remainder theorem", "combinatorics", "constructive algorithms",
	"data structures", "dfs and similar", "divide and conquer", "dp", "dsu",
	"expression parsing", "fft", "flows", "games", "geometry", "graph matchings",
	"graphs", "greedy", "hashing", "implementation", "interactive", "math",
	"matrices", "meet-in-the-middle", "number theory", "probabilities", "schedules",
	"shortest paths", "sortings", "string suffix structures", "strings", "ternary search",
	"trees", "two pointers",
}

// Function to fuzzy find tags
func fuzzyFindTags(topics []string) map[string]string {
	matches := make(map[string]string)
	tagMatcher := closestmatch.New(tags, []int{2}) // 2 is the maximum number of closest matches

	for _, topic := range topics {
		// First, check for an exact match
		exactMatch := false
		for _, tag := range tags {
			if strings.EqualFold(topic, tag) { // Case-insensitive comparison
				matches[topic] = tag
				exactMatch = true
				break
			}
		}

		// If no exact match, use fuzzy matching
		if !exactMatch {
			bestMatch := tagMatcher.Closest(topic)
			matches[topic] = bestMatch
		}
	}
	return matches
}

// Struct to capture the individual problem
type Problem struct {
	ContestID   int      `json:"contestId"`
	Index       string   `json:"index"` // Problem index
	Name        string   `json:"name"`
	Type        string   `json:"type"`   // Problem type
	Points      float64  `json:"points"` // Problem points (if available)
	Rating      int      `json:"rating"` // Problem rating
	Tags        []string `json:"tags"`
	SolvedCount int      `json:"solvedCount"` // Solved count, merged from problemStatistics
}

// Function to fetch problems from Codeforces API
func fetchProblemsByTags(tags []string) ([]Problem, error) {
	tagsString := strings.Join(tags, ";")
	url := fmt.Sprintf("https://codeforces.com/api/problemset.problems?tags=%s", tagsString)

	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	// Check if the response status is OK
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch problems: %s", response.Status)
	}

	// Read the response body
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON response into the ProblemSetResponse struct
	var apiResponse struct {
		Status string `json:"status"`
		Result struct {
			Problems          []Problem `json:"problems"`
			ProblemStatistics []struct {
				ContestID   int    `json:"contestId"`
				Index       string `json:"index"`
				SolvedCount int    `json:"solvedCount"`
			} `json:"problemStatistics"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, err
	}

	// Merge the problemStatistics into problems
	for i := range apiResponse.Result.Problems {
		apiResponse.Result.Problems[i].SolvedCount = apiResponse.Result.ProblemStatistics[i].SolvedCount
	}

	return apiResponse.Result.Problems, nil
}

// Helper function to get color based on rating
func getColorByRating(rating int) string {
	switch {
	case rating >= 1000 && rating <= 1199:
		return "\033[1;90m" // Grey
	case rating >= 1200 && rating <= 1399:
		return "\033[1;32m" // Green
	case rating >= 1400 && rating <= 1599:
		return "\033[1;36m" // Cyan
	case rating >= 1600 && rating <= 1899:
		return "\033[1;34m" // Blue
	case rating >= 1900 && rating <= 2099:
		return "\033[1;35m" // Pink
	case rating >= 2100 && rating <= 2299:
		return "\033[1;33m" // Orange
	case rating >= 2300 && rating <= 2399:
		return "\033[38;5;208m" // Dark Orange
	case rating >= 2400 && rating <= 2599:
		return "\033[1;31m" // Red
	default:
		return "\033[0m" // Reset
	}
}

// Function to filter problems by rating range
func filterProblemsByRating(problems []Problem, minRating, maxRating int) []Problem {
	filtered := []Problem{}
	for _, problem := range problems {
		if problem.Rating >= minRating && problem.Rating <= maxRating {
			filtered = append(filtered, problem)
		}
	}
	return filtered
}

// Function to sort problems based on user input
func sortProblems(problems []Problem, order string) {
	if order == "d" {
		// Sort by rating descending, then by solved count descending
		sort.SliceStable(problems, func(i, j int) bool {
			if problems[i].Rating == problems[j].Rating {
				return problems[i].SolvedCount > problems[j].SolvedCount
			}
			return problems[i].Rating > problems[j].Rating
		})
	} else {
		// Sort by rating ascending, then by solved count ascending
		sort.SliceStable(problems, func(i, j int) bool {
			if problems[i].Rating == problems[j].Rating {
				return problems[i].SolvedCount < problems[j].SolvedCount
			}
			return problems[i].Rating < problems[j].Rating
		})
	}
}

// Function to fetch solved problems for the user
func fetchSolvedProblems() (map[string]struct{}, error) {
	username := "shauncodes"
	url := fmt.Sprintf("https://codeforces.com/api/user.status?handle=%s", username)

	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch user status: %s", response.Status)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Status string `json:"status"`
		Result []struct {
			Problem struct {
				ContestID int    `json:"contestId"`
				Index     string `json:"index"`
			} `json:"problem"`
			Verdict string `json:"verdict"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, err
	}

	solvedProblems := make(map[string]struct{})
	for _, submission := range apiResponse.Result {
		if submission.Verdict == "OK" {
			problemKey := fmt.Sprintf("%d_%s", submission.Problem.ContestID, submission.Problem.Index)
			solvedProblems[problemKey] = struct{}{}
		}
	}

	return solvedProblems, nil
}

func clearScreen() {
	switch runtime.GOOS {
	case "linux", "darwin":
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	default:
		// As a fallback, print 100 empty lines
		for i := 0; i < 100; i++ {
			fmt.Println()
		}
	}
}

// Helper function to truncate strings
func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..." // Keep the ellipsis
	}
	return s
}

// Updated displayPage function
func displayPage(problems []Problem, solvedProblems map[string]struct{}, page, pageSize int) {
	// Get terminal size
	terminalWidth, terminalHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		fmt.Println("Error getting terminal size:", err)
		return
	}

	// Calculate how many rows can be displayed (leaving space for headers, etc.)
	availableRows := terminalHeight - 4 // Adjust this based on header/footer space
	if availableRows < 1 {
		availableRows = 1
	}

	// Calculate start and end indices for pagination
	start := (page - 1) * pageSize
	end := start + availableRows
	if end > len(problems) {
		end = len(problems)
	}

	clearScreen()

	// Determine column widths based on terminal width
	const (
		colContestIDWidth   = 12
		colProblemWidth     = 10
		colNameWidth        = 40 // Will be adjusted based on terminal width
		colSolvedCountWidth = 12
		colRatingWidth      = 12
		colSolvedWidth      = 12
	)

	// Calculate available width for the name column
	nameWidth := terminalWidth - (colContestIDWidth + colProblemWidth + colSolvedCountWidth + colRatingWidth + colSolvedWidth + 5) // Subtracting spaces

	// Ensure the name width is within reasonable limits
	if nameWidth < 10 {
		nameWidth = 10
	}

	// Print header
	fmt.Printf("\033[1;34m%-12s %-10s %-*s %-12s %-12s %-12s\033[0m\n", "Contest ID", "Problem", nameWidth, "Name", "Solved Count", "Rating", "Solved")

	// Print content rows
	for _, problem := range problems[start:end] {
		link := fmt.Sprintf("https://codeforces.com/contest/%d/problem/%s", problem.ContestID, problem.Index)
		ratingColor := getColorByRating(problem.Rating)
		problemKey := fmt.Sprintf("%d_%s", problem.ContestID, problem.Index)

		solvedMarker := "No"
		solvedColor := "\033[1;31m" // Red for "No"
		if _, exists := solvedProblems[problemKey]; exists {
			solvedMarker = "Yes"
			solvedColor = "\033[1;32m" // Green for "Yes"
		}

		// Truncate the problem name if it's too long
		truncatedName := truncateString(problem.Name, nameWidth)

		fmt.Printf("\033[1;32m%-12d\033[0m \033[1;31m%-10s\033[0m \033]8;;%s\033\\%-*s\033]8;;\033\\ %-12d %s%-12d %s%s\033[0m\n",
			problem.ContestID, problem.Index, link, nameWidth, truncatedName, problem.SolvedCount, ratingColor, problem.Rating, solvedColor, solvedMarker)
	}
	fmt.Printf("\nPage %d of %d\n", page, (len(problems)+pageSize-1)/pageSize)
}

func printRow(columns []string, widths []int, isHeader bool) {
	for i, col := range columns {
		if isHeader {
			fmt.Printf("│\033[1;34m %-*s\033[0m", widths[i]-1, col)
		} else {
			fmt.Printf("│ %-*s", widths[i]-1, col)
		}
	}
	fmt.Println("│")
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Enter the topics (comma-separated):")
	scanner.Scan()
	input := scanner.Text()

	words := strings.Split(input, ",")
	var topics []string
	// Iterate over the words
	for _, word := range words {
		trimmedWord := strings.TrimSpace(word) // Trim spaces
		if trimmedWord != "" {                 // Filter out empty strings
			topics = append(topics, trimmedWord)
		}
	}

	matches := fuzzyFindTags(topics)
	fmt.Println("Fuzzy Matches:")
	for topic, match := range matches {
		fmt.Printf("%s -> %s\n", topic, match)
	}

	tagsToSearch := make([]string, 0)
	for _, match := range matches {
		tagsToSearch = append(tagsToSearch, match)
	}

	// Concurrent fetching of problems and solved problems
	var wg sync.WaitGroup
	wg.Add(2)

	var problems []Problem
	var solvedProblems map[string]struct{}
	var problemsErr, solvedErr error

	go func() {
		defer wg.Done()
		problems, problemsErr = fetchProblemsByTags(tagsToSearch)
	}()

	go func() {
		defer wg.Done()
		solvedProblems, solvedErr = fetchSolvedProblems()
	}()

	// Get user input while fetching is in progress
	var minRating, maxRating int
	var order string

	fmt.Println("Enter min and max rating | Sort Order (a/d):")
	fmt.Scanf("%d %d %s", &minRating, &maxRating, &order)

	// Wait for fetching to complete
	wg.Wait()

	// Check for errors after fetching
	if problemsErr != nil {
		fmt.Println("Error fetching problems:", problemsErr)
		return
	}
	if solvedErr != nil {
		fmt.Println("Error fetching solved problems:", solvedErr)
		return
	}

	// Filter and sort problems
	filteredProblems := filterProblemsByRating(problems, minRating, maxRating)
	sortProblems(filteredProblems, order)

	pageSize := 20 // Number of problems to display per page
	currentPage := 1
	if err := keyboard.Open(); err != nil {
		panic(err)
	}
	defer keyboard.Close()

	for {
		displayPage(filteredProblems, solvedProblems, currentPage, pageSize)

		fmt.Println("\nPress 'n' for next page, 'p' for previous page, 'j' to jump to a page, or 'q' to quit:")

		char, key, err := keyboard.GetKey()
		if err != nil {
			panic(err)
		}

		switch char {
		case 'n':
			if currentPage*pageSize < len(filteredProblems) {
				currentPage++
			}
		case 'p':
			if currentPage > 1 {
				currentPage--
			}
		case 'q':
			return
		case 'j':
			keyboard.Close()
			fmt.Print("Enter page number: ")
			var pageInput string
			fmt.Scanln(&pageInput)
			pageNum, err := strconv.Atoi(pageInput)
			if err == nil && pageNum >= 1 && pageNum <= (len(filteredProblems)+pageSize-1)/pageSize {
				currentPage = pageNum
			} else {
				fmt.Println("Invalid page number. Press any key to continue.")
				keyboard.GetKey()
			}
			if err := keyboard.Open(); err != nil {
				panic(err)
			}
		}

		if key == keyboard.KeyCtrlC {
			break
		}
	}
}
