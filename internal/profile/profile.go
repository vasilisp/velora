package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vasilisp/velora/internal/util"
)

type AllowedDays map[time.Weekday]struct{}

type Sport uint8

const (
	Cycling Sport = iota
	Running
)

func (s Sport) String() string {
	util.Assert(s >= Cycling && s <= Running, "invalid sport")
	return []string{"Cycling", "Running"}[s]
}

func (d AllowedDays) MarshalJSON() ([]byte, error) {
	days := make([]string, 0, len(d))
	for day := range d {
		days = append(days, day.String())
	}
	return json.Marshal(days)
}

func (d *AllowedDays) UnmarshalJSON(data []byte) error {
	var days []string
	if err := json.Unmarshal(data, &days); err != nil {
		return err
	}

	*d = make(map[time.Weekday]struct{})
	for _, dayStr := range days {
		var day time.Weekday
		switch dayStr {
		case "Monday":
			day = time.Monday
		case "Tuesday":
			day = time.Tuesday
		case "Wednesday":
			day = time.Wednesday
		case "Thursday":
			day = time.Thursday
		case "Friday":
			day = time.Friday
		case "Saturday":
			day = time.Saturday
		case "Sunday":
			day = time.Sunday
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
		days = append(days, day.String())
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

func Read() Profile {
	profilePath := filepath.Join(os.Getenv("HOME"), ".velora", "prefs.json")
	profileBytes := []byte{}

	profileBytes, err := os.ReadFile(profilePath)
	if err != nil {
		util.Fatalf("error reading profile: %v\n", err)
	}

	var p Profile
	if err := json.Unmarshal(profileBytes, &p); err != nil {
		util.Fatalf("error unmarshalling profile: %v\n", err)
	}

	return p
}

func (p Profile) AllowedDaysOfSport(sport Sport) AllowedDays {
	switch sport {
	case Cycling:
		return p.CyclingConstraints.AllowedDays
	case Running:
		return p.RunningConstraints.AllowedDays
	default:
		util.Fatalf("invalid sport: %d", sport)
	}
	panic("unreachable")
}

func (p Profile) AllowedDaysAny() AllowedDays {
	cycling := p.CyclingConstraints.AllowedDays
	running := p.RunningConstraints.AllowedDays

	result := AllowedDays{}

	for day := range cycling {
		result[day] = struct{}{}
	}

	for day := range running {
		result[day] = struct{}{}
	}

	return result
}
