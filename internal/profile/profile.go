package profile

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
	AllowedDays          AllowedDays `json:"allowed_days"`
	TrainsIndoors        bool        `json:"trains_indoors"`
	TargetDistanceDate   time.Time   `json:"target_distance_date,omitempty"`
}

func (sc SportConstraints) MarshalJSON() ([]byte, error) {
	type Alias SportConstraints

	if sc.TargetDistanceDate.IsZero() {
		return json.Marshal(&struct {
			*Alias
			TargetDistanceDate *string `json:"target_distance_date,omitempty"`
		}{
			Alias:              (*Alias)(&sc),
			TargetDistanceDate: nil,
		})
	}

	return json.Marshal(&struct {
		*Alias
		TargetDistanceDate string `json:"target_distance_date"`
	}{
		Alias:              (*Alias)(&sc),
		TargetDistanceDate: sc.TargetDistanceDate.Format("2006-01-02"),
	})
}

func (sc *SportConstraints) UnmarshalJSON(data []byte) error {
	type Alias SportConstraints

	aux := &struct {
		*Alias
		TargetDistanceDate string `json:"target_distance_date"`
	}{
		Alias: (*Alias)(sc),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.TargetDistanceDate != "" {
		date, err := time.Parse("2006-01-02", aux.TargetDistanceDate)
		if err != nil {
			return err
		}
		sc.TargetDistanceDate = date
	}

	return nil
}

type Profile struct {
	CyclingConstraints SportConstraints `json:"cycling_constraints"`
	RunningConstraints SportConstraints `json:"running_constraints"`
	FTP                uint             `json:"ftp"`
}
