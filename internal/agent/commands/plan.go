package commands

import "github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"

const modeMigrationMsg = "Mode /build is deprecated; use /agent (implementation) or /chat (web/docs). Switching to /agent."

func Plan(d Deps) error {
	d.SetMode("agent")
	if d.MutateSession != nil {
		d.MutateSession(func(s *chatstore.Session) {
			s.PlanningActive = true
		})
	}
	PrintSystem(d.Out, "Planning active — plan tools available. Mode: agent")
	return nil
}
