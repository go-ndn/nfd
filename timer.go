package main

import (
	"time"
)

const (
	LSAExpireIntv  = 15 * time.Minute
	LSAFloodIntv   = time.Minute
	LoopDetectIntv = time.Minute
	FibUpdateIntv  = 2 * time.Minute
)
