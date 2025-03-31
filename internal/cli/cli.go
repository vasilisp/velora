package cli

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/vasilisp/velora/internal/db"
	"github.com/vasilisp/velora/internal/util"
)

func addActivity(dbh *sql.DB, args []string) {
	if len(args) < 3 {
		util.Fatalf("Usage: velora add <sport> <duration> <distance>")
	}

	sportStr := args[0]
	sport, err := db.SportFromString(sportStr)
	if err != nil {
		util.Fatalf("Invalid sport: %s\n", sportStr)
	}

	duration, err := strconv.Atoi(args[1])
	if err != nil {
		util.Fatalf("invalid duration: %s\n", args[1])
	}

	distance, err := strconv.Atoi(args[2])
	if err != nil {
		util.Fatalf("invalid distance: %s\n", args[2])
	}

	activity, err := db.NewActivity(time.Now(), duration, duration, distance, sport)
	if err != nil {
		util.Fatalf("error creating activity: %v\n", err)
	}

	util.Assert(activity != nil, "add nil activity")

	if err := db.InsertActivity(dbh, *activity); err != nil {
		util.Fatalf("error inserting activity: %v\n", err)
	}
}
func formatDuration(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	if hours > 0 {
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func formatDistance(meters int) string {
	if meters >= 1000 {
		return fmt.Sprintf("%.1fkm", float64(meters)/1000)
	}
	return fmt.Sprintf("%dm", meters)
}

func showLastActivities(dbh *sql.DB) {
	fmt.Println("recent activities:")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("%-20s %-8s %-10s %s\n", "Date", "Sport", "Time", "Distance")
	fmt.Println("--------------------------------------------------")

	activities, err := db.LastActivities(dbh, 10)
	if err != nil {
		util.Fatalf("error getting last activities: %v\n", err)
	}

	for _, activity := range activities {
		fmt.Printf("%-20s %-8s %-10s %s\n",
			activity.Timestamp.Format("Jan 2, 15:04"),
			activity.Sport,
			formatDuration(activity.Duration),
			formatDistance(activity.Distance))
	}
}

func Main() {
	dbh, err := db.Init()
	if err != nil {
		util.Fatalf("Error initializing database: %v\n", err)
	}
	defer dbh.Close()

	if len(os.Args) > 1 && os.Args[1] == "add" {
		addActivity(dbh, os.Args[2:])
	}

	if len(os.Args) > 1 && os.Args[1] == "recent" {
		showLastActivities(dbh)
	}
}
