package main

import "time"

type Subtitle struct {
	Index int
	Start time.Duration
	End   time.Duration
	Text  string
}
