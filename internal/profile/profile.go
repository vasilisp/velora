package profile

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vasilisp/velora/internal/util"
)

type AllowedDays map[util.DayOfWeek]struct{}

func (d AllowedDays) MarshalJSON() ([]byte, error) {
	days := make([]string, 0, len(d))
	for day := range d {
		dayStr, err := day.String()
		if err != nil {
			return nil, fmt.Errorf("error marshaling day: %v", err)
		}
		days = append(days, dayStr)
	}
	return json.Marshal(days)
}

func (d *AllowedDays) UnmarshalJSON(data []byte) error {
	var days []string
	if err := json.Unmarshal(data, &days); err != nil {
		return err
	}

	*d = make(map[util.DayOfWeek]struct{})
	for _, dayStr := range days {
		var day util.DayOfWeek
		switch dayStr {
		case "Monday":
			day = util.Monday
		case "Tuesday":
			day = util.Tuesday
		case "Wednesday":
			day = util.Wednesday
		case "Thursday":
			day = util.Thursday
		case "Friday":
			day = util.Friday
		case "Saturday":
			day = util.Saturday
		case "Sunday":
			day = util.Sunday
		default:
			return fmt.Errorf("invalid day: %s", dayStr)
		}
		(*d)[day] = struct{}{}
	}

	return nil
}

func (d AllowedDays) Complement() AllowedDays {
	complement := make(AllowedDays)
	for _, day := range util.AllDays {
		if _, ok := d[day]; !ok {
			complement[day] = struct{}{}
		}
	}
	return complement
}

func (d AllowedDays) String() string {
	var days []string
	for day := range d {
		dayStr, err := day.String()
		if err != nil {
			util.Fatalf("error getting day string: %v", err)
		}
		days = append(days, dayStr)
	}
	return strings.Join(days, ", ")
}

type SportConstraints struct {
	TargetWeeklyDistance uint        `json:"target_weekly_distance"`
	TargetDistance       uint        `json:"target_distance"`
	Days                 AllowedDays `json:"days"`
	TrainsIndoors        bool        `json:"trains_indoors"`
}

type Profile struct {
	CyclingConstraints SportConstraints `json:"cycling_constraints"`
	RunningConstraints SportConstraints `json:"running_constraints"`
	FTP                uint             `json:"ftp"`
}

func (p Profile) Describe() string {
	var parts []string

	if p.CyclingConstraints.TargetWeeklyDistance > 0 {
		parts = append(parts, fmt.Sprintf("- I target %s of cycling per week.",
			util.FormatDistance(int(p.CyclingConstraints.TargetWeeklyDistance))))
	}

	if len(p.CyclingConstraints.Days) > 0 {
		parts = append(parts, fmt.Sprintf("- I can cycle on %s.", p.CyclingConstraints.Days.String()))
	} else {
		parts = append(parts, "- I do not cycle.")
	}

	if len(p.CyclingConstraints.Days) < 7 {
		parts = append(parts, fmt.Sprintf("- I cannot cycle on %s.", p.CyclingConstraints.Days.Complement().String()))
	}

	if p.CyclingConstraints.TargetDistance > 0 {
		parts = append(parts, fmt.Sprintf("- I am training for a %s ride.",
			util.FormatDistance(int(p.CyclingConstraints.TargetDistance))))
	}

	if p.RunningConstraints.TargetWeeklyDistance > 0 {
		parts = append(parts, fmt.Sprintf("- I target %s of running per week.",
			util.FormatDistance(int(p.RunningConstraints.TargetWeeklyDistance))))
	}

	if len(p.RunningConstraints.Days) > 0 {
		parts = append(parts, fmt.Sprintf("- I can run on %s.", p.RunningConstraints.Days.String()))
	} else {
		parts = append(parts, "- I do not run.")
	}

	if len(p.RunningConstraints.Days) < 7 {
		parts = append(parts, fmt.Sprintf("- I cannot run on %s.", p.RunningConstraints.Days.Complement().String()))
	}

	if p.RunningConstraints.TargetDistance > 0 {
		parts = append(parts, fmt.Sprintf("- I am training for a %s run.",
			util.FormatDistance(int(p.RunningConstraints.TargetDistance))))
	}

	if p.FTP > 0 {
		parts = append(parts, fmt.Sprintf("- My FTP is %d watts.", p.FTP))
	}

	if p.CyclingConstraints.TrainsIndoors {
		parts = append(parts, "- I can do cycling training indoors (on a turbo trainer).")
	} else {
		parts = append(parts, "- I cannot do cycling training indoors (on a turbo trainer).")
	}

	if p.RunningConstraints.TrainsIndoors {
		parts = append(parts, "- I can do running training indoors (on a treadmill).")
	} else {
		parts = append(parts, "- I cannot do running training indoors (on a treadmill).")
	}

	return strings.Join(parts, "\n")
}
