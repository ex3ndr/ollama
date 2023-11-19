package progress

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/jmorganca/ollama/format"
	"golang.org/x/term"
)

type Stats struct {
	rate      int64
	value     int64
	remaining time.Duration
}

type Bar struct {
	message      string
	messageWidth int

	maxValue     int64
	initialValue int64
	currentValue int64

	started time.Time
	stopped time.Time

	maxBuckets int
	buckets    []bucket
}

type bucket struct {
	updated time.Time
	value   int64
}

func NewBar(message string, maxValue, initialValue int64) *Bar {
	return &Bar{
		message:      message,
		messageWidth: -1,
		maxValue:     maxValue,
		initialValue: initialValue,
		currentValue: initialValue,
		started:      time.Now(),
		maxBuckets:   10,
	}
}

func (b *Bar) String() string {
	termWidth, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil {
		termWidth = 80
	}

	var pre, mid, suf strings.Builder

	if len(b.message) > 0 {
		message := strings.TrimSpace(b.message)
		if b.messageWidth > 0 && len(message) > b.messageWidth {
			message = message[:b.messageWidth]
		}

		fmt.Fprintf(&pre, "%s", message)
		if padding := b.messageWidth - pre.Len(); padding > 0 {
			pre.WriteString(strings.Repeat(" ", padding))
		}

		pre.WriteString(" ")
	}

	fmt.Fprintf(&pre, "%3.0f%% ", math.Floor(b.percent()))

	suf.WriteRune('(')

	progress := format.HumanBytes(b.maxValue)
	if b.stopped.IsZero() {
		progress = fmt.Sprintf("%s/%s", format.HumanBytes(b.currentValue), format.HumanBytes(b.maxValue))
	}

	fmt.Fprintf(&suf, "%s", progress)

	rate := b.rate()
	if b.stopped.IsZero() {
		fmt.Fprintf(&suf, ", %s/s", format.HumanBytes(int64(rate)))
	}

	suf.WriteRune(')')

	if b.stopped.IsZero() {
		var remaining time.Duration
		if rate > 0 {
			remaining = time.Duration(int64(float64(b.maxValue-b.currentValue)/rate)) * time.Second
		}

		fmt.Fprintf(&suf, " [%s:%s]", b.elapsed(), remaining)
	}

	// add 3 extra spaces: 2 boundary characters and 1 space at the end
	f := termWidth - pre.Len() - suf.Len() - 3
	n := int(float64(f) * b.percent() / 100)

	mid.WriteString("▕")

	if n > 0 {
		mid.WriteString(strings.Repeat("█", n))
	}

	if f-n > 0 {
		mid.WriteString(strings.Repeat(" ", f-n))
	}

	mid.WriteString("▏")

	return pre.String() + mid.String() + suf.String()
}

func (b *Bar) Set(value int64) {
	if value >= b.maxValue {
		value = b.maxValue
	}

	b.currentValue = value
	if b.currentValue >= b.maxValue {
		b.stopped = time.Now()
	}

	// throttle bucket updates to 1 per second
	if len(b.buckets) == 0 || time.Since(b.buckets[len(b.buckets)-1].updated) > time.Second {
		b.buckets = append(b.buckets, bucket{
			updated: time.Now(),
			value:   value,
		})

		if len(b.buckets) > b.maxBuckets {
			b.buckets = b.buckets[1:]
		}
	}
}

func (b *Bar) percent() float64 {
	if b.maxValue > 0 {
		return float64(b.currentValue) / float64(b.maxValue) * 100
	}

	return 0
}

func (b *Bar) rate() float64 {
	if !b.stopped.IsZero() {
		return (float64(b.currentValue) - float64(b.initialValue)) / b.elapsed().Seconds()
	}

	switch len(b.buckets) {
	case 0:
		return 0
	case 1:
		return float64(b.buckets[0].value-b.initialValue) / b.buckets[0].updated.Sub(b.started).Seconds()
	default:
		first, last := b.buckets[0], b.buckets[len(b.buckets)-1]
		return (float64(last.value) - float64(first.value)) / last.updated.Sub(first.updated).Seconds()
	}
}

func (b *Bar) elapsed() time.Duration {
	elapsed := time.Since(b.started)
	if !b.stopped.IsZero() {
		elapsed = b.stopped.Sub(b.started)
	}

	return elapsed.Round(time.Second)
}
