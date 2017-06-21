package benchmarker

import (
	"fmt"
	"io"
	"log"
	"math"
	"sort"
	"sync"
)

// Stat represents one statistical measurement.
type Stat struct {
	Label     []byte
	Value     float64
	IsWarm    bool
	IsPartial bool
}

// Init safely initializes a (cold) Stat while minimizing heap allocations.
func (s *Stat) Init(label []byte, value float64) {
	s.Label = s.Label[:0] // clear
	s.Label = append(s.Label, label...)
	s.Value = value
	s.IsWarm = false
}

// InitWarm safely initializes a warm Stat while minimizing heap allocations.
func (s *Stat) InitWarm(label []byte, value float64) {
	s.Label = s.Label[:0] // clear
	s.Label = append(s.Label, label...)
	s.Value = value
	s.IsWarm = true
}

// GetStatPool returns a sync.Pool for Stat
func GetStatPool() sync.Pool {
	return sync.Pool{
		New: func() interface{} {
			return &Stat{
				Label: make([]byte, 0, 1024),
				Value: 0.0,
			}
		},
	}
}

// StatGroup collects simple streaming statistics.
type StatGroup struct {
	Min  float64
	Max  float64
	Mean float64
	Sum  float64

	// used for stddev calculations
	m      float64
	s      float64
	StdDev float64

	Count int64
}

// Push updates a StatGroup with a new value.
func (s *StatGroup) Push(n float64) {
	if s.Count == 0 {
		s.Min = n
		s.Max = n
		s.Mean = n
		s.Count = 1
		s.Sum = n

		s.m = n
		s.s = 0.0
		s.StdDev = 0.0
		return
	}

	if n < s.Min {
		s.Min = n
	}
	if n > s.Max {
		s.Max = n
	}

	s.Sum += n

	// constant-space mean update:
	sum := s.Mean*float64(s.Count) + n
	s.Mean = sum / float64(s.Count+1)

	s.Count++

	oldM := s.m
	s.m += (n - oldM) / float64(s.Count)
	s.s += (n - oldM) * (n - s.m)
	s.StdDev = math.Sqrt(s.s / (float64(s.Count) - 1.0))
}

// String makes a simple description of a StatGroup.
func (s *StatGroup) String() string {
	return fmt.Sprintf("min: %f, max: %f, mean: %f, count: %d, sum: %f, stddev: %f", s.Min, s.Max, s.Mean, s.Count, s.Sum, s.StdDev)
}

func (s *StatGroup) Write(w io.Writer) error {
	minRate := 1e3 / s.Min
	meanRate := 1e3 / s.Mean
	maxRate := 1e3 / s.Max

	_, err := fmt.Fprintf(w, "min: %8.2fms (%7.2f/sec), mean: %8.2fms (%7.2f/sec), max: %7.2fms (%6.2f/sec), stddev: %8.2f, sum: %5.1fsec \n", s.Min, minRate, s.Mean, meanRate, s.Max, maxRate, s.StdDev, s.Sum/1e3)
	return err
}

// WriteStatGroupMap writes a map of StatGroups in an ordered fashion by
// key that they are stored by
func WriteStatGroupMap(w io.Writer, statGroups map[string]*StatGroup) {
	maxKeyLength := 0
	keys := make([]string, 0, len(statGroups))
	for k := range statGroups {
		if len(k) > maxKeyLength {
			maxKeyLength = len(k)
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := statGroups[k]
		paddedKey := fmt.Sprintf("%s", k)
		for len(paddedKey) < maxKeyLength {
			paddedKey += " "
		}

		_, err := fmt.Fprintf(w, "%s:\n", paddedKey)
		if err != nil {
			log.Fatal(err)
		}

		err = v.Write(w)
		if err != nil {
			log.Fatal(err)
		}
	}
}
