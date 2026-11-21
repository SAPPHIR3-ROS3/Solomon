package chatstore

import (
	"fmt"
	"strconv"
)

type messageStream struct {
	name     string
	messages *[]Message
}

func streamsFor(s *Session) []messageStream {
	streams := []messageStream{{name: "main", messages: &s.Messages}}
	for i := range s.Branches {
		streams = append(streams, messageStream{fmt.Sprintf("branch/%d", i), &s.Branches[i].Messages})
	}
	for i := range s.UncompactedRaw {
		snapshot := &s.UncompactedRaw[i]
		prefix := "snapshot/" + strconv.Itoa(i)
		streams = append(streams, messageStream{prefix, &snapshot.Messages})
		for j := range snapshot.Branches {
			streams = append(streams, messageStream{
				fmt.Sprintf("%s/branch/%d", prefix, j), &snapshot.Branches[j].Messages,
			})
		}
	}
	return streams
}

func copySessionStructure(s *Session) Session {
	result := *s
	result.Branches = append([]BranchSegment(nil), s.Branches...)
	result.UncompactedRaw = append([]UncompactedDump(nil), s.UncompactedRaw...)
	for index := range result.UncompactedRaw {
		result.UncompactedRaw[index].Branches = append([]BranchSegment(nil), result.UncompactedRaw[index].Branches...)
	}
	return result
}
