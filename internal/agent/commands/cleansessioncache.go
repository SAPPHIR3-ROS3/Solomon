package commands

import (
	"fmt"

	"github.com/SAPPHIR3-ROS3/Solomon/internal/chatstore"
)

func CleanSessionCache(d Deps) error {
	s := d.Session()
	if s == nil {
		return fmt.Errorf("no session")
	}
	broken, patched, emptied := chatstore.RepairSessionMalformedImages(s)
	if d.PersistSession != nil {
		if err := d.PersistSession(); err != nil {
			return err
		}
	}
	fmt.Fprintf(d.Out, "[cleansessioncache] dropped %d bad image attachments; sanitized %d user prompt(s); %d bare-image stubs replaced with \"(image omitted)\"\n",
		broken, patched, emptied)
	return nil
}
