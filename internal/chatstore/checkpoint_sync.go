package chatstore

func MessageCheckpointTagVisible(m Message) bool {
	if m.CheckpointSeq < 0 {
		return false
	}
	if m.CpSeqSet {
		return true
	}
	return m.CheckpointSeq > 0
}

func FinishSessionLoad(s *Session) {
	if s == nil {
		return
	}
	MigrateLegacyCheckpointsToBase0(s)
	ReconcileCheckpointLast(s)
}

func MigrateLegacyCheckpointsToBase0(s *Session) {
	if s == nil || s.CheckpointCP0 {
		return
	}
	hasLegacyPositive := false
	for _, m := range s.Messages {
		if m.CheckpointSeq > 0 {
			hasLegacyPositive = true
			break
		}
	}
	if !hasLegacyPositive {
		for i := range s.MainOrphans {
			for _, om := range s.MainOrphans[i].Messages {
				if om.CheckpointSeq > 0 {
					hasLegacyPositive = true
					break
				}
			}
			if hasLegacyPositive {
				break
			}
		}
	}
	if !hasLegacyPositive {
		s.CheckpointCP0 = true
		return
	}
	for i := range s.Messages {
		if s.Messages[i].CheckpointSeq > 0 {
			s.Messages[i].CheckpointSeq--
			s.Messages[i].CpSeqSet = true
		}
	}
	for i := range s.MainOrphans {
		if s.MainOrphans[i].ForkAtInclusive > 0 {
			s.MainOrphans[i].ForkAtInclusive--
		}
		for j := range s.MainOrphans[i].Messages {
			if s.MainOrphans[i].Messages[j].CheckpointSeq > 0 {
				s.MainOrphans[i].Messages[j].CheckpointSeq--
				s.MainOrphans[i].Messages[j].CpSeqSet = true
			}
		}
	}
	if s.ForkChildCount != nil {
		nm := make(map[int]int, len(s.ForkChildCount))
		for k, v := range s.ForkChildCount {
			nk := k
			if k > 0 {
				nk = k - 1
			}
			nm[nk] += v
		}
		s.ForkChildCount = nm
	}
	s.CheckpointCP0 = true
}

func ReconcileCheckpointLast(s *Session) {
	if s == nil {
		return
	}
	if len(s.Messages) == 0 {
		s.CheckpointLast = -1
		return
	}
	max := -1
	has := false
	for _, m := range s.Messages {
		if !MessageCheckpointTagVisible(m) {
			continue
		}
		has = true
		if m.CheckpointSeq > max {
			max = m.CheckpointSeq
		}
	}
	if !has {
		s.CheckpointLast = -1
		return
	}
	s.CheckpointLast = max
}
