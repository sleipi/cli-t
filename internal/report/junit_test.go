package report

import (
	"bytes"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type parsedFailure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

type parsedCase struct {
	Name      string         `xml:"name,attr"`
	Classname string         `xml:"classname,attr"`
	Time      string         `xml:"time,attr"`
	Failure   *parsedFailure `xml:"failure"`
	Error     *parsedFailure `xml:"error"`
	Skipped   *parsedFailure `xml:"skipped"`
	SystemOut string         `xml:"system-out"`
	SystemErr string         `xml:"system-err"`
}

type parsedSuite struct {
	Name     string       `xml:"name,attr"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Errors   int          `xml:"errors,attr"`
	Skipped  int          `xml:"skipped,attr"`
	Time     string       `xml:"time,attr"`
	Cases    []parsedCase `xml:"testcase"`
}

type parsedSuites struct {
	XMLName  xml.Name      `xml:"testsuites"`
	Tests    int           `xml:"tests,attr"`
	Failures int           `xml:"failures,attr"`
	Errors   int           `xml:"errors,attr"`
	Skipped  int           `xml:"skipped,attr"`
	Time     string        `xml:"time,attr"`
	Suites   []parsedSuite `xml:"testsuite"`
}

func writeAndParse(t *testing.T, suites []Suite) (got parsedSuites, raw string) {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteJUnit(&buf, suites); err != nil {
		t.Fatalf("WriteJUnit: %v", err)
	}
	if err := xml.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("report is not valid XML: %v\n%s", err, buf.String())
	}
	return got, buf.String()
}

func TestWriteJUnit_CountsAndCases(t *testing.T) {
	suites := []Suite{{
		Name: "a.clitest",
		Time: 1500 * time.Millisecond,
		Cases: []Case{
			{Name: "passes", Time: 12 * time.Millisecond},
			{Name: "fails", Time: 8 * time.Millisecond, Failures: []string{"exit: expected 0, got 1", "stdout: expected hello"}, Stdout: "out", Stderr: "err"},
			{Name: "skipped", Skipped: true, SkipMessage: "not ready"},
		},
	}}

	got, raw := writeAndParse(t, suites)

	if !strings.HasPrefix(raw, xml.Header) {
		t.Errorf("expected XML header, got %q", raw[:min(len(raw), 40)])
	}
	if got.Tests != 3 || got.Failures != 1 || got.Skipped != 1 || got.Errors != 0 {
		t.Errorf("unexpected totals: %+v", got)
	}
	s := got.Suites[0]
	if s.Name != "a.clitest" || s.Tests != 3 || s.Failures != 1 || s.Skipped != 1 || s.Time != "1.500" {
		t.Errorf("unexpected suite: %+v", s)
	}
	if s.Cases[0].Name != "passes" || s.Cases[0].Classname != "a.clitest" || s.Cases[0].Time != "0.012" || s.Cases[0].Failure != nil {
		t.Errorf("unexpected passing case: %+v", s.Cases[0])
	}
	f := s.Cases[1].Failure
	if f == nil || f.Message != "exit: expected 0, got 1" || !strings.Contains(f.Body, "stdout: expected hello") {
		t.Errorf("unexpected failure: %+v", f)
	}
	if s.Cases[1].SystemOut != "out" || s.Cases[1].SystemErr != "err" {
		t.Errorf("unexpected system-out/err: %+v", s.Cases[1])
	}
	if s.Cases[2].Skipped == nil || s.Cases[2].Skipped.Message != "not ready" {
		t.Errorf("unexpected skipped case: %+v", s.Cases[2])
	}
}

func TestWriteJUnit_ParseErrorBecomesErrorCase(t *testing.T) {
	got, _ := writeAndParse(t, []Suite{{Name: "broken.clitest", Error: "line 3: unknown directive"}})

	s := got.Suites[0]
	if s.Tests != 1 || s.Errors != 1 || got.Errors != 1 {
		t.Errorf("unexpected counts: suite %+v, totals %+v", s, got)
	}
	if s.Cases[0].Name != "parse" || s.Cases[0].Error == nil || s.Cases[0].Error.Message != "line 3: unknown directive" || s.Cases[0].Error.Body != "line 3: unknown directive" {
		t.Errorf("unexpected error case: %+v", s.Cases[0])
	}
}

func TestWriteJUnit_EscapesSpecialAndControlCharacters(t *testing.T) {
	suites := []Suite{{
		Name:  "esc.clitest",
		Cases: []Case{{Name: `echo "<a & b>"`, Failures: []string{`stdout == "<x>"`}, Stdout: "\x1b[31mred\x1b[0m\x00"}},
	}}

	got, _ := writeAndParse(t, suites)

	c := got.Suites[0].Cases[0]
	if c.Name != `echo "<a & b>"` || c.Failure.Message != `stdout == "<x>"` {
		t.Errorf("special characters not round-tripped: %+v", c)
	}
	if !strings.Contains(c.SystemOut, "red") {
		t.Errorf("expected stdout text to survive, got %q", c.SystemOut)
	}
}

func TestWriteJUnit_EmptyRun(t *testing.T) {
	got, _ := writeAndParse(t, nil)
	if got.Tests != 0 || len(got.Suites) != 0 {
		t.Errorf("expected empty report, got %+v", got)
	}
}

func TestCreate_CreatesParentDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "build", "reports", "junit.xml")
	f, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_ = f.Close()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected report file: %v", err)
	}
}

func TestCreate_FailsWhenPathIsADirectory(t *testing.T) {
	if _, err := Create(t.TempDir()); err == nil {
		t.Fatal("expected error when the path is a directory")
	}
}

func TestCreate_FailsWhenParentIsAFile(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(filepath.Join(blocker, "report.xml")); err == nil {
		t.Fatal("expected error when parent is a file")
	}
}

func TestWriteJUnit_TotalTimeSumsSuites(t *testing.T) {
	got, _ := writeAndParse(t, []Suite{
		{Name: "a.clitest", Time: 1200 * time.Millisecond},
		{Name: "b.clitest", Time: 300 * time.Millisecond},
	})

	if got.Time != "1.500" {
		t.Errorf("expected total time 1.500, got %q", got.Time)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestWriteJUnit_ReturnsWriteError(t *testing.T) {
	if err := WriteJUnit(failingWriter{}, nil); err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("expected write error, got %v", err)
	}
}
