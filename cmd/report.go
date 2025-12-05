package cmd

import (
	"os"
	"strconv"

	"github.com/cli/go-gh"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/ssulei7/gh-dormant-users/internal/activity"
	dateUtil "github.com/ssulei7/gh-dormant-users/internal/date"
	"github.com/ssulei7/gh-dormant-users/internal/limiter"
	"github.com/ssulei7/gh-dormant-users/internal/repository"
	"github.com/ssulei7/gh-dormant-users/internal/users"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate a report",
	Run:   generateDormantUserReport,
}

func generateDormantUserReport(cmd *cobra.Command, args []string) {
	// First, get all users in an organization using the gh module
	orgName, _ := cmd.Flags().GetString("org-name")
	email, _ := cmd.Flags().GetBool("email")
	date, _ := cmd.Flags().GetString("date")
	page, _ := cmd.Flags().GetInt("page")

	client, err := gh.RESTClient(nil)
	if err != nil {
		pterm.Error.Printf("Failed to create REST client: %v\n", err)
		os.Exit(1)
	}

	// Detect rate limit for this token and configure limiter
	limiter.DetectRateLimit(client)

	// Validate date is no longer than 3 months, and turn into an ISO string
	isDateValid := dateUtil.ValidateDate(date)
	if !isDateValid {
		pterm.Error.Println("Date must be within the last 3 months")
		os.Exit(1)
	}

	// Validate page number
	if page < 0 {
		pterm.Error.Println("Page number must be a positive integer")
		os.Exit(1)
	}

	// Convert date to iso 8601 format
	isoDate := dateUtil.GetISODate(date)
	allUsers := users.GetOrganizationUsers(orgName, email, client)

	// Apply pagination if page is specified
	var usersToProcess users.Users
	const usersPerPage = 100
	totalUsers := len(allUsers)

	if page > 0 {
		// Calculate start and end indices for the page
		startIdx := (page - 1) * usersPerPage
		endIdx := startIdx + usersPerPage

		// Handle case where page exceeds available users
		if startIdx >= totalUsers {
			pterm.Warning.Printf("Page %d exceeds available users (total: %d users). No users to process.\n", page, totalUsers)
			os.Exit(0)
		}

		// Adjust end index if it exceeds total users
		if endIdx > totalUsers {
			endIdx = totalUsers
		}

		usersToProcess = allUsers[startIdx:endIdx]
		pterm.Info.Printf("Processing page %d (users %d-%d of %d)\n", page, startIdx+1, endIdx, totalUsers)
	} else {
		usersToProcess = allUsers
		pterm.Info.Printf("Processing all %d users\n", totalUsers)
	}

	repositories := repository.GetOrgRepositories(orgName, client)

	activityTypes, _ := cmd.Flags().GetStringSlice("activity-types")

	// Now, check for activity in the organization's repositories
	box := pterm.DefaultBox.WithTitle("Organization Info").
		WithLeftPadding(1).
		WithRightPadding(1).
		WithBottomPadding(1).
		WithTopPadding(1)
	box.Printfln("Number of users: %v\nNumber of repositories: %v", len(usersToProcess), len(repositories))
	pterm.Info.Println("Checking for activity...")
	activity.CheckActivity(usersToProcess, orgName, repositories, isoDate, client, activityTypes)
	activity.GenerateBarChartOfActiveUsers()

	// Update CSV filename to include page number if specified
	csvFilename := orgName + "-dormant-users.csv"
	if page > 0 {
		csvFilename = orgName + "-dormant-users-page-" + strconv.Itoa(page) + ".csv"
	}
	activity.GenerateUserReportCSV(usersToProcess, csvFilename)
}
