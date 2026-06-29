package commands

import (
	"io"
	"time"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
)

func ExportChatBasenameForTest(sess *chatstore.Session) string {
	return exportChatBasename(sess)
}

func PlanExportPathForTest(rootDir string, day time.Time, base string, rejectIfExists bool) (exportPathPlan, error) {
	return planExportPath(rootDir, day, base, rejectIfExists)
}

func WriteMarkdownExportForTest(w io.Writer, meta markdownExportMeta, sess *chatstore.Session, showUsage bool) error {
	return writeMarkdownExport(w, meta, sess, showUsage)
}

func MarkdownExportMetaForTest(title, projectRoot, model string) markdownExportMeta {
	return markdownExportMeta{
		Title:       title,
		ExportedAt:  time.Now().UTC(),
		ProjectRoot: projectRoot,
		Model:       model,
		ExportRoot:  "/tmp/export",
	}
}
