// Package report writes machine-readable test reports for CI systems.
package report

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Suite is one .clitest file in a report.
type Suite struct {
	Name  string
	Time  time.Duration
	Cases []Case
	Error string // set when the file could not be loaded or parsed
}

// Case is one entry in a report. It failed when Failures is not empty.
type Case struct {
	Name        string
	Time        time.Duration
	Failures    []string
	Skipped     bool
	SkipMessage string
	Stdout      string
	Stderr      string
}

type junitMessage struct {
	Message string `xml:"message,attr,omitempty"`
	Body    string `xml:",chardata"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitMessage `xml:"failure,omitempty"`
	Error     *junitMessage `xml:"error,omitempty"`
	Skipped   *junitMessage `xml:"skipped,omitempty"`
	SystemOut string        `xml:"system-out,omitempty"`
	SystemErr string        `xml:"system-err,omitempty"`
}

type junitSuite struct {
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Failures int         `xml:"failures,attr"`
	Errors   int         `xml:"errors,attr"`
	Skipped  int         `xml:"skipped,attr"`
	Time     string      `xml:"time,attr"`
	Cases    []junitCase `xml:"testcase"`
}

type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Errors   int          `xml:"errors,attr"`
	Skipped  int          `xml:"skipped,attr"`
	Time     string       `xml:"time,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

// Create creates or truncates the report file at path, creating parent directories.
func Create(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // CI artifact, read by other users (e.g. host runner after a container run)
		return nil, fmt.Errorf("creating report: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644) //nolint:gosec // CI artifact, read by other users (e.g. host runner after a container run)
	if err != nil {
		return nil, fmt.Errorf("creating report: %w", err)
	}
	return f, nil
}

// WriteJUnit writes suites as JUnit XML to w.
func WriteJUnit(w io.Writer, suites []Suite) error {
	doc := junitSuites{}
	var total time.Duration
	for _, s := range suites {
		js := toJUnitSuite(s)
		doc.Tests += js.Tests
		doc.Failures += js.Failures
		doc.Errors += js.Errors
		doc.Skipped += js.Skipped
		total += s.Time
		doc.Suites = append(doc.Suites, js)
	}
	doc.Time = seconds(total)

	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding junit report: %w", err)
	}
	content := append([]byte(xml.Header), out...)
	content = append(content, '\n')
	if _, err := w.Write(content); err != nil {
		return fmt.Errorf("writing junit report: %w", err)
	}
	return nil
}

func toJUnitSuite(s Suite) junitSuite {
	js := junitSuite{Name: s.Name, Time: seconds(s.Time)}
	if s.Error != "" {
		js.Tests, js.Errors = 1, 1
		js.Cases = []junitCase{{
			Name:      "parse",
			Classname: s.Name,
			Time:      seconds(0),
			Error:     &junitMessage{Message: s.Error, Body: s.Error},
		}}
		return js
	}
	for _, c := range s.Cases {
		jc := junitCase{
			Name:      c.Name,
			Classname: s.Name,
			Time:      seconds(c.Time),
			SystemOut: c.Stdout,
			SystemErr: c.Stderr,
		}
		switch {
		case c.Skipped:
			js.Skipped++
			jc.Skipped = &junitMessage{Message: c.SkipMessage}
		case len(c.Failures) > 0:
			js.Failures++
			jc.Failure = &junitMessage{Message: c.Failures[0], Body: strings.Join(c.Failures, "\n")}
		}
		js.Tests++
		js.Cases = append(js.Cases, jc)
	}
	return js
}

func seconds(d time.Duration) string {
	return fmt.Sprintf("%.3f", d.Seconds())
}
