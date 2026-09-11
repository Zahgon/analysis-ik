package help

import (
	"fmt"
	"os"
	"time"
)

// SleepType is the unit a Sleep duration is expressed in.
type SleepType int

// The units Sleep accepts.
const (
	MSEC SleepType = iota
	SEC
	MIN
	HOUR
)

// Sleep blocks for num units of the given type. An unknown type reports the
// error on stderr and returns without sleeping, as the original did.
func Sleep(sleepType SleepType, num int) {
	switch sleepType {
	case MSEC:
		time.Sleep(time.Duration(int32(num)) * time.Millisecond)
	case SEC:
		time.Sleep(time.Duration(int32(num)*1000) * time.Millisecond)
	case MIN:
		time.Sleep(time.Duration(int32(num)*60*1000) * time.Millisecond)
	case HOUR:
		time.Sleep(time.Duration(int32(num)*60*60*1000) * time.Millisecond)
	default:
		fmt.Fprintln(os.Stderr, "输入类型错误，应为MSEC,SEC,MIN,HOUR之一")
	}
}
