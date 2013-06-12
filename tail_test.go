// Copyright (c) 2013 ActiveState Software Inc. All rights reserved.

// TODO:
//  * repeat all the tests with Poll:true

package tail

import (
	"./watch"
	_ "fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func init() {
	// Clear the temporary test directory
	err := os.RemoveAll(".test")
	if err != nil {
		panic(err)
	}
}

func TestMustExist(t *testing.T) {
	tail, err := TailFile("/no/such/file", Config{Follow: true, MustExist: true})
	if err == nil {
		t.Error("MustExist:true is violated")
		tail.Stop()
	}
	tail, err = TailFile("/no/such/file", Config{Follow: true, MustExist: false})
	if err != nil {
		t.Error("MustExist:false is violated")
	}
	tail.Stop()
	_, err = TailFile("README.md", Config{Follow: true, MustExist: true})
	if err != nil {
		t.Error("MustExist:true on an existing file is violated")
	}
	tail.Stop()
}

func TestMaxLineSize(_t *testing.T) {
	t := NewTailTest("maxlinesize", _t)
	t.CreateFile("test.txt", "hello\nworld\nfin\nhe")
	tail := t.StartTail("test.txt", Config{Follow: true, Location: -1, MaxLineSize: 3})
	go t.VerifyTailOutput(tail, []string{"hel", "lo", "wor", "ld", "fin", "he"})

	// Delete after a reasonable delay, to give tail sufficient time
	// to read all lines.
	<-time.After(100 * time.Millisecond)
	t.RemoveFile("test.txt")
	tail.Stop()
}

func TestLocationFull(_t *testing.T) {
	t := NewTailTest("location-full", _t)
	t.CreateFile("test.txt", "hello\nworld\n")
	tail := t.StartTail("test.txt", Config{Follow: true, Location: -1})
	go t.VerifyTailOutput(tail, []string{"hello", "world"})

	// Delete after a reasonable delay, to give tail sufficient time
	// to read all lines.
	<-time.After(100 * time.Millisecond)
	t.RemoveFile("test.txt")
	tail.Stop()
}

func TestLocationFullDontFollow(_t *testing.T) {
	t := NewTailTest("location-full-dontfollow", _t)
	t.CreateFile("test.txt", "hello\nworld\n")
	tail := t.StartTail("test.txt", Config{Follow: false, Location: -1})
	go t.VerifyTailOutput(tail, []string{"hello", "world"})

	// Add more data only after reasonable delay.
	<-time.After(100 * time.Millisecond)
	t.AppendFile("test.txt", "more\ndata\n")
	<-time.After(100 * time.Millisecond)

	tail.Stop()
}

func TestLocationEnd(_t *testing.T) {
	t := NewTailTest("location-end", _t)
	t.CreateFile("test.txt", "hello\nworld\n")
	tail := t.StartTail("test.txt", Config{Follow: true, Location: 0})
	go t.VerifyTailOutput(tail, []string{"more", "data"})

	<-time.After(100 * time.Millisecond)
	t.AppendFile("test.txt", "more\ndata\n")

	// Delete after a reasonable delay, to give tail sufficient time
	// to read all lines.
	<-time.After(100 * time.Millisecond)
	t.RemoveFile("test.txt")
	tail.Stop()
}

func _TestReOpen(_t *testing.T, poll bool) {
	var name string
	if poll {
		name = "reopen-polling"
	} else {
		name = "reopen-inotify"
	}
	t := NewTailTest(name, _t)
	t.CreateFile("test.txt", "hello\nworld\n")
	tail := t.StartTail(
		"test.txt",
		Config{Follow: true, ReOpen: true, Poll: poll, Location: -1})

	go t.VerifyTailOutput(tail, []string{"hello", "world", "more", "data", "endofworld"})

	// deletion must trigger reopen
	<-time.After(100 * time.Millisecond)
	t.RemoveFile("test.txt")
	<-time.After(100 * time.Millisecond)
	t.CreateFile("test.txt", "more\ndata\n")

	// rename must trigger reopen
	<-time.After(100 * time.Millisecond)
	t.RenameFile("test.txt", "test.txt.rotated")
	<-time.After(100 * time.Millisecond)
	t.CreateFile("test.txt", "endofworld")

	// Delete after a reasonable delay, to give tail sufficient time
	// to read all lines.
	<-time.After(100 * time.Millisecond)
	t.RemoveFile("test.txt")
	<-time.After(100 * time.Millisecond)

	// Do not bother with stopping as it could kill the tomb during
	// the reading of data written above. Timings can vary based on
	// test environment.
	// tail.Stop()
}

// The use of polling file watcher could affect file rotation
// (detected via renames), so test these explicitly.

func TestReOpenInotify(_t *testing.T) {
	_TestReOpen(_t, false)
}

func TestReOpenPolling(_t *testing.T) {
	_TestReOpen(_t, true)
}

func _TestReSeek(_t *testing.T, poll bool) {
	var name string
	if poll {
		name = "reseek-polling"
	} else {
		name = "reseek-inotify"
	}
	t := NewTailTest(name, _t)
	t.CreateFile("test.txt", "a really long string goes here\nhello\nworld\n")
	tail := t.StartTail(
		"test.txt",
		Config{Follow: true, ReOpen: false, Poll: poll, Location: -1})

	go t.VerifyTailOutput(tail, []string{
		"a really long string goes here", "hello", "world", "h311o", "w0r1d", "endofworld"})

	// truncate now
	<-time.After(100 * time.Millisecond)
	t.TruncateFile("test.txt", "h311o\nw0r1d\nendofworld\n")

	// Delete after a reasonable delay, to give tail sufficient time
	// to read all lines.
	<-time.After(100 * time.Millisecond)
	t.RemoveFile("test.txt")

	// Do not bother with stopping as it could kill the tomb during
	// the reading of data written above. Timings can vary based on
	// test environment.
	// tail.Stop()
}

// The use of polling file watcher could affect file rotation
// (detected via renames), so test these explicitly.

func TestReSeekInotify(_t *testing.T) {
	_TestReSeek(_t, false)
}

func TestReSeekPolling(_t *testing.T) {
	_TestReSeek(_t, true)
}

// Test library

type TailTest struct {
	Name string
	path string
	*testing.T
}

func NewTailTest(name string, t *testing.T) TailTest {
	tt := TailTest{name, ".test/" + name, t}
	err := os.MkdirAll(tt.path, os.ModeTemporary|0700)
	if err != nil {
		tt.Fatal(err)
	}

	// Use a smaller poll duration for faster test runs. Keep it below
	// 100ms (which value is used as common delays for tests)
	watch.POLL_DURATION = 5 * time.Millisecond

	return tt
}

func (t TailTest) CreateFile(name string, contents string) {
	err := ioutil.WriteFile(t.path+"/"+name, []byte(contents), 0600)
	if err != nil {
		t.Fatal(err)
	}
}

func (t TailTest) RemoveFile(name string) {
	err := os.Remove(t.path + "/" + name)
	if err != nil {
		t.Fatal(err)
	}
}

func (t TailTest) RenameFile(oldname string, newname string) {
	oldname = t.path + "/" + oldname
	newname = t.path + "/" + newname
	err := os.Rename(oldname, newname)
	if err != nil {
		t.Fatal(err)
	}
}

func (t TailTest) AppendFile(name string, contents string) {
	f, err := os.OpenFile(t.path+"/"+name, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, err = f.WriteString(contents)
	if err != nil {
		t.Fatal(err)
	}
}

func (t TailTest) TruncateFile(name string, contents string) {
	f, err := os.OpenFile(t.path+"/"+name, os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, err = f.WriteString(contents)
	if err != nil {
		t.Fatal(err)
	}
}

func (t TailTest) StartTail(name string, config Config) *Tail {
	tail, err := TailFile(t.path+"/"+name, config)
	if err != nil {
		t.Fatal(err)
	}
	return tail
}

func (t TailTest) VerifyTailOutput(tail *Tail, lines []string) {
	for idx, line := range lines {
		tailedLine, ok := <-tail.Lines
		if !ok {
			err := tail.Err()
			if err != nil {
				t.Errorf("tail ended with error: %v", err)
			}
			t.Fatalf("tail ended early; expecting more: %v", lines[idx:])
		}
		if tailedLine == nil {
			t.Fatalf("tail.Lines returned nil; not possible")
		}
		if tailedLine.Text != line {
			t.Fatalf("mismatch; %s (actual) != %s (expected)",
				tailedLine.Text, line)
		}
	}
	line, ok := <-tail.Lines
	if ok {
		t.Fatalf("more content from tail: %s", line.Text)
	}
}
