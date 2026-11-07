package chatstore

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type summaryEntry struct {
	info     os.FileInfo
	size     int64
	modified time.Time
	session  Session
}

var summaries = struct {
	sync.Mutex
	entries map[string]summaryEntry
}{entries: map[string]summaryEntry{}}

func ListSummaries(projectHex string, limit int) ([]*Session, error) {
	directory, err := ChatsDir(projectHex)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []*Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		name := filepath.Join(directory, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		summaries.Lock()
		cached, ok := summaries.entries[name]
		summaries.Unlock()
		if !ok || !os.SameFile(cached.info, info) || cached.size != info.Size() || !cached.modified.Equal(info.ModTime()) {
			file, err := os.Open(name)
			if err != nil {
				continue
			}
			session, err := readSummary(json.NewDecoder(file))
			file.Close()
			if err != nil {
				continue
			}
			cached = summaryEntry{info: info, size: info.Size(), modified: info.ModTime(), session: session}
			summaries.Lock()
			if len(summaries.entries) >= 512 {
				clear(summaries.entries)
			}
			summaries.entries[name] = cached
			summaries.Unlock()
		}
		session := cached.session
		result = append(result, &session)
	}
	sort.Slice(result, func(i, j int) bool { return SessionActivityTime(result[i]).After(SessionActivityTime(result[j])) })
	if limit >= 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func readSummary(decoder *json.Decoder) (Session, error) {
	var session Session
	token, err := decoder.Token()
	if err != nil {
		return session, err
	}
	if token != json.Delim('{') {
		return session, io.ErrUnexpectedEOF
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return session, err
		}
		switch key {
		case "id":
			err = decoder.Decode(&session.ID)
		case "title":
			err = decoder.Decode(&session.Title)
		case "created_at":
			err = decoder.Decode(&session.CreatedAt)
		case "last_message_at":
			var latest time.Time
			err = decoder.Decode(&latest)
			if latest.After(session.LastMessageAt) {
				session.LastMessageAt = latest
			}
		case "last_user_message_at":
			var latest time.Time
			err = decoder.Decode(&latest)
			if latest.After(session.LastUserMessageAt) {
				session.LastUserMessageAt = latest
			}
		case "messages":
			var latest, activity time.Time
			latest, activity, err = summaryMessageTime(decoder)
			if activity.After(session.LastMessageAt) {
				session.LastMessageAt = activity
			}
			if latest.After(session.LastUserMessageAt) {
				session.LastUserMessageAt = latest
			}
		default:
			err = skipSummaryValue(decoder)
		}
		if err != nil {
			return session, err
		}
	}
	_, err = decoder.Token()
	return session, err
}

func summaryMessageTime(decoder *json.Decoder) (time.Time, time.Time, error) {
	var latest, activity time.Time
	token, err := decoder.Token()
	if err != nil || token == nil {
		return latest, activity, err
	}
	if token != json.Delim('[') {
		return latest, activity, io.ErrUnexpectedEOF
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return latest, activity, err
		}
		if token != json.Delim('{') {
			return latest, activity, io.ErrUnexpectedEOF
		}
		var role string
		var created time.Time
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return latest, activity, err
			}
			switch key {
			case "role":
				err = decoder.Decode(&role)
			case "created_at":
				err = decoder.Decode(&created)
			default:
				err = skipSummaryValue(decoder)
			}
			if err != nil {
				return latest, activity, err
			}
		}
		if _, err = decoder.Token(); err != nil {
			return latest, activity, err
		}
		if created.After(activity) {
			activity = created
		}
		if role == "user" && created.After(latest) {
			latest = created
		}
	}
	_, err = decoder.Token()
	return latest, activity, err
}

func skipSummaryValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delimiter != '[' && delimiter != '{' {
		return io.ErrUnexpectedEOF
	}
	for decoder.More() {
		if delimiter == '{' {
			if _, err := decoder.Token(); err != nil {
				return err
			}
		}
		if err := skipSummaryValue(decoder); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}
