package tailer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	audit "k8s.io/apiserver/pkg/apis/audit"
)

type Tailer struct {
	logDir     string
	offsetFile string
	eventCh    chan *audit.Event
}

func New(logDir, offsetFile string, eventCh chan *audit.Event) *Tailer {
	return &Tailer{
		logDir:     logDir,
		offsetFile: offsetFile,
		eventCh:    eventCh,
	}
}

func (t *Tailer) Run() {
	logFile := filepath.Join(t.logDir, "audit.log")
	offset := t.loadOffset()

	for {
		err := t.tailFile(logFile, &offset)
		if err != nil {
			log.Printf("Error tailing %s: %v, retrying in 5s", logFile, err)
			time.Sleep(5 * time.Second)
			continue
		}
		time.Sleep(1 * time.Second)
	}
}

func (t *Tailer) tailFile(path string, offset *int64) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	// Log rotation: file is smaller than our offset
	if info.Size() < *offset {
		log.Printf("Log rotation detected, resetting offset from %d to 0", *offset)
		*offset = 0
	}

	if *offset > 0 {
		if _, err := f.Seek(*offset, io.SeekStart); err != nil {
			return fmt.Errorf("seek: %w", err)
		}
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event audit.Event
		if err := json.Unmarshal(line, &event); err != nil {
			log.Printf("Failed to parse audit event: %v", err)
			continue
		}

		t.eventCh <- &event
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner: %w", err)
	}

	currentPos, err := f.Seek(0, io.SeekCurrent)
	if err == nil {
		*offset = currentPos
		t.saveOffset(*offset)
	}

	return nil
}

func (t *Tailer) loadOffset() int64 {
	data, err := os.ReadFile(t.offsetFile)
	if err != nil {
		return 0
	}
	offset, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return 0
	}
	return offset
}

func (t *Tailer) saveOffset(offset int64) {
	_ = os.WriteFile(t.offsetFile, []byte(strconv.FormatInt(offset, 10)), 0644)
}
