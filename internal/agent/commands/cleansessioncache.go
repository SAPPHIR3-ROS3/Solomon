package commands

import (
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func CleanSessionCache(d Deps) error {
	if d.MutateSession == nil {
		return fmt.Errorf("no session")
	}
	var broken, patched, emptied, neutralized int
	var fragsBefore, fragsAfter int
	d.MutateSession(func(s *chatstore.Session) {
		if s == nil {
			return
		}
		fragsBefore = chatstore.SessionImgFragmentCount(s)
		broken, patched, emptied, neutralized = chatstore.RepairSessionMalformedImages(s)
		fragsAfter = chatstore.SessionImgFragmentCount(s)
	})
	if d.PersistSession != nil {
		if err := d.PersistSession(); err != nil {
			return err
		}
	}
	PrintSystemf(d.Out, "[cleansessioncache] dropped %d bad image attachments; sanitized %d user prompt(s); %d bare-image stubs; stripped %d field(s) (assistant/reasoning/tool/tool-call args); [img- fragments %d→%d",
		broken, patched, emptied, neutralized, fragsBefore, fragsAfter)
	if neutralized == 0 && fragsBefore == fragsAfter && fragsAfter > 0 {
		PrintSystemf(d.Out, "[cleansessioncache] note: %d [img- substring(s) remain (e.g. inside words or already scrubbed each prompt on load); user [img-N] with a valid paste are kept on purpose", fragsAfter)
	}
	if neutralized == 0 && fragsBefore == 0 {
		PrintSystemf(d.Out, "[cleansessioncache] session already clean (repair also runs automatically before each input line)")
	}
	return nil
}
