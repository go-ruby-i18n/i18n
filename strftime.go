// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

package i18n

import (
	"strconv"
	"strings"
	"time"
)

// Temporal is the date/time value Localize formats. rbgo's Date / Time / DateTime
// objects implement it (the seam): the library never constructs a clock value,
// it only reads the broken-down fields a host hands it. A plain Go time.Time
// satisfies the contract through TimeValue.
type Temporal interface {
	Year() int
	Month() int // 1..12
	Day() int   // 1..31
	Hour() int  // 0..23
	Min() int   // 0..59
	Sec() int   // 0..59
	Nsec() int  // 0..999999999
	// Wday is the 0..6 day-of-week, Sunday == 0 (Ruby's Time#wday).
	Wday() int
	// Yday is the 1..366 day-of-year (Ruby's Time#yday).
	Yday() int
	// ZoneOffset is the offset east of UTC in seconds (Ruby's Time#utc_offset).
	ZoneOffset() int
	// ZoneName is the abbreviated zone name, "" when unknown (Ruby's Time#zone).
	ZoneName() string
	// HasTime reports whether this value carries a clock component (a Time /
	// DateTime) versus a bare Date; it selects the "time" vs "date" format tree
	// and whether %p is meaningful, mirroring object.respond_to?(:sec).
	HasTime() bool
}

// TimeValue adapts a Go time.Time to Temporal so callers can localize standard
// library times directly. It always reports HasTime() == true.
type TimeValue struct{ T time.Time }

func (t TimeValue) Year() int  { return t.T.Year() }
func (t TimeValue) Month() int { return int(t.T.Month()) }
func (t TimeValue) Day() int   { return t.T.Day() }
func (t TimeValue) Hour() int  { return t.T.Hour() }
func (t TimeValue) Min() int   { return t.T.Minute() }
func (t TimeValue) Sec() int   { return t.T.Second() }
func (t TimeValue) Nsec() int  { return t.T.Nanosecond() }
func (t TimeValue) Wday() int  { return int(t.T.Weekday()) }
func (t TimeValue) Yday() int  { return t.T.YearDay() }
func (t TimeValue) ZoneOffset() int {
	_, off := t.T.Zone()
	return off
}
func (t TimeValue) ZoneName() string {
	name, _ := t.T.Zone()
	if name == "UTC" || name == "" {
		// Ruby's %Z for a UTC time is "UTC"; for a fixed offset it is "".
		return name
	}
	return name
}
func (t TimeValue) HasTime() bool { return true }

// DateValue adapts a Go time.Time to Temporal as a bare Date (no clock): HasTime
// is false, so the "date" format tree is used and clock directives read zero.
type DateValue struct{ T time.Time }

func (d DateValue) Year() int       { return d.T.Year() }
func (d DateValue) Month() int      { return int(d.T.Month()) }
func (d DateValue) Day() int        { return d.T.Day() }
func (d DateValue) Hour() int       { return 0 }
func (d DateValue) Min() int        { return 0 }
func (d DateValue) Sec() int        { return 0 }
func (d DateValue) Nsec() int       { return 0 }
func (d DateValue) Wday() int       { return int(d.T.Weekday()) }
func (d DateValue) Yday() int       { return d.T.YearDay() }
func (d DateValue) ZoneOffset() int { return 0 }
func (d DateValue) ZoneName() string {
	return ""
}
func (d DateValue) HasTime() bool { return false }

// strftime renders obj per a strftime(3)-style format, reproducing the directives
// MRI's Time/Date#strftime emit (the locale-aware %a/%A/%b/%B/%p/%P are already
// substituted from translation data by the localize layer before this runs, so
// here they fall through to MRI's own English output for completeness). Flags
// "-" (no padding) and "_" (space padding) and a width on %N are supported.
func strftime(obj Temporal, format string) string {
	var b strings.Builder
	i := 0
	for i < len(format) {
		if format[i] != '%' {
			b.WriteByte(format[i])
			i++
			continue
		}
		// Parse: % [flags] [width] [^] conv
		j := i + 1
		if j >= len(format) {
			b.WriteByte('%')
			break
		}
		flag := byte(0)
		upcase := false
		for j < len(format) && (format[j] == '-' || format[j] == '_' || format[j] == '0' || format[j] == '^') {
			switch format[j] {
			case '-', '_', '0':
				flag = format[j]
			case '^':
				upcase = true
			}
			j++
		}
		width := 0
		hasWidth := false
		for j < len(format) && format[j] >= '0' && format[j] <= '9' {
			width = width*10 + int(format[j]-'0')
			hasWidth = true
			j++
		}
		// A ":" before z selects the colon offset form (%:z -> "+02:00").
		colon := false
		if j < len(format) && format[j] == ':' {
			colon = true
			j++
		}
		if j >= len(format) {
			// Trailing flags with no directive: emit verbatim.
			b.WriteString(format[i:])
			break
		}
		conv := format[j]
		if conv == 'z' && colon {
			b.WriteString(formatOffset(obj.ZoneOffset(), true))
			i = j + 1
			continue
		}
		out := strftimeDirective(obj, conv, flag, width, hasWidth)
		if out == "\x00" { // unknown directive: emit literally
			b.WriteString(format[i : j+1])
		} else {
			if upcase {
				out = strings.ToUpper(out)
			}
			b.WriteString(out)
		}
		i = j + 1
	}
	return b.String()
}

var enDayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
var enAbbrDayNames = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
var enMonthNames = []string{"", "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
var enAbbrMonthNames = []string{"", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// strftimeDirective renders a single directive. A "\x00" return signals "unknown".
func strftimeDirective(o Temporal, conv, flag byte, width int, hasWidth bool) string {
	pad := func(n, defWidth int) string {
		s := strconv.Itoa(n)
		w := defWidth
		switch flag {
		case '-':
			return s
		case '_':
			for len(s) < w {
				s = " " + s
			}
			return s
		default:
			for len(s) < w {
				s = "0" + s
			}
			return s
		}
	}
	hour := o.Hour()
	switch conv {
	case 'Y':
		return strconv.Itoa(o.Year())
	case 'y':
		return pad(((o.Year()%100)+100)%100, 2)
	case 'C':
		return pad(o.Year()/100, 2)
	case 'm':
		return pad(o.Month(), 2)
	case 'd':
		return pad(o.Day(), 2)
	case 'e':
		return spacePad(o.Day(), 2, flag)
	case 'H':
		return pad(hour, 2)
	case 'k':
		return spacePad(hour, 2, flag)
	case 'I':
		h12 := hour % 12
		if h12 == 0 {
			h12 = 12
		}
		return pad(h12, 2)
	case 'l':
		h12 := hour % 12
		if h12 == 0 {
			h12 = 12
		}
		return spacePad(h12, 2, flag)
	case 'M':
		return pad(o.Min(), 2)
	case 'S':
		return pad(o.Sec(), 2)
	case 'L':
		w := 3
		if hasWidth {
			w = width
		}
		return fracSeconds(o.Nsec(), w)
	case 'N':
		w := 9
		if hasWidth {
			w = width
		}
		return fracSeconds(o.Nsec(), w)
	case 'p':
		if hour < 12 {
			return "AM"
		}
		return "PM"
	case 'P':
		if hour < 12 {
			return "am"
		}
		return "pm"
	case 'j':
		return pad(o.Yday(), 3)
	case 'u':
		w := o.Wday()
		if w == 0 {
			w = 7
		}
		return strconv.Itoa(w)
	case 'w':
		return strconv.Itoa(o.Wday())
	case 'A':
		return enDayNames[o.Wday()]
	case 'a':
		return enAbbrDayNames[o.Wday()]
	case 'B':
		return enMonthNames[o.Month()]
	case 'b', 'h':
		return enAbbrMonthNames[o.Month()]
	case 'z':
		return formatOffset(o.ZoneOffset(), false)
	case 'Z':
		return o.ZoneName()
	case 'n':
		return "\n"
	case 't':
		return "\t"
	case '%':
		return "%"
	default:
		return "\x00"
	}
}

// spacePad renders n to a minimum width using the space-default directives' flag
// semantics: '-' suppresses padding, '0' zero-pads, anything else space-pads
// (Ruby's %e/%k/%l default to space, but honour an explicit %0e etc.).
func spacePad(n, width int, flag byte) string {
	s := strconv.Itoa(n)
	switch flag {
	case '-':
		return s
	case '0':
		for len(s) < width {
			s = "0" + s
		}
		return s
	default:
		for len(s) < width {
			s = " " + s
		}
		return s
	}
}

// fracSeconds renders the fractional-second value at the requested width (digits
// after the decimal point), as %L/%N do.
func fracSeconds(nsec, width int) string {
	s := strconv.Itoa(nsec)
	for len(s) < 9 {
		s = "0" + s
	}
	if width <= 9 {
		return s[:width]
	}
	for len(s) < width {
		s += "0"
	}
	return s
}

// formatOffset renders a UTC offset given seconds east of UTC. colon selects the
// "+02:00" form (%:z) over the default "+0200" form (%z).
func formatOffset(sec int, colon bool) string {
	sign := "+"
	if sec < 0 {
		sign = "-"
		sec = -sec
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	hs := strconv.Itoa(h)
	if len(hs) < 2 {
		hs = "0" + hs
	}
	ms := strconv.Itoa(m)
	if len(ms) < 2 {
		ms = "0" + ms
	}
	if colon {
		return sign + hs + ":" + ms
	}
	return sign + hs + ms
}
