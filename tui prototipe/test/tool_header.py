import os
import re
import subprocess
import unittest
import terminal as terminal_support
from screen import cell_width

ROOT=terminal_support.ROOT

class ToolHeaderTests(unittest.TestCase):
    def test_tool_header_does_not_overflow_terminal_cells(self):
        for name in ("atelier","orbit","paper","mission"):
            out=subprocess.check_output([str(ROOT/name),"--snapshot","120","40","tool_call"])
            plain=re.sub(rb"\x1b\[[0-9;]*m",b"",out).decode()
            for row in plain.splitlines():
                if "●" in row or "○" in row:
                    width=cell_width(row)
                    print(f"{name} header terminal cells: {width}",flush=True)
                    self.assertLessEqual(width,120)

if __name__=="__main__":
    unittest.main()
