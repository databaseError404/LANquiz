package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const scoreProgressionFilePath = "data/score-progression.txt"

var defaultScoreProgression = []int{0, 100, 200, 300, 500, 700, 1000, 2000, 3000, 5000, 10000, 15000, 20000, 30000}

func ensureAndLoadScoreProgression(path string) {
	if err := ensureScoreProgressionFile(path); err != nil {
		scoreProgression = append([]int(nil), defaultScoreProgression...)
		return
	}

	vals, err := parseScoreProgressionFile(path)
	if err != nil || len(vals) == 0 {
		scoreProgression = append([]int(nil), defaultScoreProgression...)
		return
	}

	scoreProgression = vals
}

func ensureScoreProgressionFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, v := range defaultScoreProgression {
		if _, err := fmt.Fprintln(f, v); err != nil {
			return err
		}
	}
	return nil
}

func parseScoreProgressionFile(path string) ([]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vals := make([]int, 0, 16)
	s := bufio.NewScanner(f)
	lineNo := 0
	prev := -1
	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		v, err := strconv.Atoi(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: not an integer", lineNo)
		}
		if v < 0 {
			return nil, fmt.Errorf("line %d: negative value", lineNo)
		}
		if len(vals) > 0 && v <= prev {
			return nil, fmt.Errorf("line %d: values must be strictly increasing", lineNo)
		}
		vals = append(vals, v)
		prev = v
	}

	if err := s.Err(); err != nil {
		return nil, err
	}

	return vals, nil
}
