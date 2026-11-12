import fcntl
import json
import os
from pathlib import Path
import pty
import struct
import signal
import subprocess
import tempfile
import termios
import unittest
import terminal as terminal_support
from screen import Screen

ROOT = terminal_support.ROOT


class MouseTests(unittest.TestCase):
    drain = terminal_support.TerminalTests.drain

    def update_screen(self, master, screen, duration=0.2):
        output = self.drain(master, duration)
        screen.feed(output)
        return output

    def click(self, master, screen, label):
        x, y = screen.find(label)
        x += 1
        y += 1
        os.write(master, f"\x1b[<0;{x};{y}M\x1b[<0;{x};{y}m".encode())
        self.update_screen(master, screen)

    def test_card_click_after_scrolling_and_resize(self):
        for name in ("atelier", "orbit", "paper", "mission"):
            with self.subTest(layout=name), tempfile.TemporaryDirectory(prefix="solomon-tui-mouse-") as folder:
                env = dict(os.environ, SOLOMON_TUI_CONFIG=str(Path(folder) / "interface.json"), TERM="xterm-256color", COLORTERM="truecolor")
                master, slave = pty.openpty()
                fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
                process = subprocess.Popen([str(ROOT / name), "--artifacts"], stdin=slave, stdout=slave, stderr=slave, env=env)
                screen = Screen()
                try:
                    self.update_screen(master, screen, 0.4)
                    for _ in range(8):
                        if "Decision summary" in screen.text():
                            break
                        os.write(master, b"\x1b[<65;40;15M")
                        self.update_screen(master, screen)
                    self.assertIn("Decision summary", screen.text())
                    self.click(master, screen, "Decision summary")
                    self.assertIn("Ctrl+D expand", screen.text())
                    self.click(master, screen, "Decision summary")
                    self.assertIn("existing binary file endpoint", screen.text())
                    x, y = screen.find("F1 Atelier")
                    pressed = b"\x1b[M" + bytes((32, x + 33, y + 33))
                    released = b"\x1b[M" + bytes((35, x + 33, y + 33))
                    os.write(master, pressed + released)
                    self.update_screen(master, screen)
                    self.assertIn("You: ", screen.text())
                    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 32, 100, 0, 0))
                    process.send_signal(signal.SIGWINCH)
                    screen = Screen(100, 32)
                    self.update_screen(master, screen)
                    self.click(master, screen, "F4 Mission")
                    self.assertIn("gpt-6.1-sol", screen.text())
                    self.assertNotIn("777;zone", screen.text())
                    os.write(master, b"\x03")
                    self.update_screen(master, screen)
                    process.wait(timeout=3)
                    self.assertEqual(process.returncode, 0)
                finally:
                    if process.poll() is None:
                        process.kill()
                        process.wait()
                    os.close(master)
                    os.close(slave)

    def test_complete_tool_call_in_component_map(self):
        with tempfile.TemporaryDirectory(prefix="solomon-tool-card-") as folder:
            env = dict(os.environ, SOLOMON_TUI_CONFIG=str(Path(folder) / "interface.json"), TERM="xterm-256color", COLORTERM="truecolor")
            master, slave = pty.openpty()
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
            process = subprocess.Popen([str(ROOT / "laboratory"), "--components"], stdin=slave, stdout=slave, stderr=slave, env=env)
            screen = Screen()
            try:
                self.update_screen(master, screen, 0.4)
                self.click(master, screen, "Tool call")
                self.assertIn("Tool: orchestrate", screen.text())
                self.assertIn("Source", screen.text())
                dot_x,dot_y=screen.find("●")
                self.assertEqual(dot_x,25)
                tool_x,tool_y=screen.find("Tool: orchestrate")
                self.assertEqual(tool_x,dot_x+3)
                self.assertEqual(screen.cells[tool_y][dot_x]," ")
                source_x,source_y=screen.find("Source")
                self.assertEqual(screen.cells[source_y][dot_x]," ")
                self.assertEqual(screen.cells[source_y][-5],"╮")
                self.assertIn("CODE", screen.text())
                self.click(master, screen, "Leggere editor e API dei file")
                self.assertNotIn("Source", screen.text())
                self.click(master, screen, "Leggere editor e API dei file")
                self.assertIn("Source", screen.text())
                self.click(master, screen, "Source")
                self.assertNotIn("func main", screen.text())
                self.assertIn("Result", screen.text())
                self.click(master, screen, "Result")
                self.assertIn("▸", next(row for row in screen.text().splitlines() if "Result" in row))
                self.click(master, screen, "Source")
                self.assertIn("func main", screen.text())
                self.assertIn("▸", next(row for row in screen.text().splitlines() if "Result" in row))
                self.click(master, screen, "Source")
                self.click(master, screen, "Result")
                self.assertIn("▸", next(row for row in screen.text().splitlines() if "Source" in row))
                self.assertNotIn("▸", next(row for row in screen.text().splitlines() if "Result" in row))
                self.click(master, screen, "Source")
                os.write(master, b"\t")
                self.update_screen(master, screen)
                self.assertNotIn("Source", screen.text())
                self.assertNotIn("Tool: orchestrate",screen.text())
                self.assertNotIn("func main",screen.text())
                self.assertNotIn("Execution",screen.text())
                self.assertNotIn("Result",screen.text().split("Tool call · collapsed")[-1])
                os.write(master, b"\t")
                self.update_screen(master, screen)
                self.assertNotIn("Running",screen.text())
                phases=set()
                for _ in range(6):
                    self.update_screen(master,screen,0.2)
                    if "○" in screen.text(): phases.add("empty")
                    if "●" in screen.text(): phases.add("full")
                self.assertEqual(phases,{"empty","full"})
                self.assertIn("Source", screen.text())
                os.write(master, b"\t")
                self.update_screen(master, screen)
                self.assertIn("Tool call · failed", screen.text())
                self.assertIn("Tool execution failed", screen.text())
                self.assertNotIn("Running", screen.text())
                os.write(master, b"\x03")
                self.update_screen(master, screen)
                process.wait(timeout=3)
                self.assertEqual(process.returncode, 0)
            finally:
                if process.poll() is None:
                    process.kill()
                    process.wait()
                os.close(master)
                os.close(slave)

    def test_default_clicks_and_live_mouse_setting(self):
        with tempfile.TemporaryDirectory(prefix="solomon-tui-mouse-") as folder:
            config = Path(folder) / "interface.json"
            env = dict(os.environ, SOLOMON_TUI_CONFIG=str(config), TERM="xterm-256color", COLORTERM="truecolor")
            subprocess.run([str(ROOT / "laboratory"), "--init-config"], env=env, capture_output=True, check=True)
            master, slave = pty.openpty()
            original = termios.tcgetattr(slave)
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
            process = subprocess.Popen([str(ROOT / "mission")], stdin=slave, stdout=slave, stderr=slave, env=env)
            screen = Screen()
            try:
                output = self.update_screen(master, screen, 0.5)
                self.assertIn(b"\x1b[?1002h", output)
                self.click(master, screen, "gpt-6.1-sol")
                self.assertIn("Select model", screen.text())
                self.click(master, screen, "Codex")
                self.assertNotIn("Select model", screen.text())
                self.assertIn("Codex ▾", screen.text())
                self.click(master, screen, "None ▾")
                self.assertIn("Reasoning effort", screen.text())
                self.click(master, screen, "Low")
                self.assertIn("Low ▾", screen.text())
                self.click(master, screen, "Agent")
                self.assertIn("Chat", screen.text())
                self.click(master, screen, "components")
                self.assertIn("COMPONENT MAP", screen.text())
                self.click(master, screen, "Image")
                self.assertIn("actual image pixels", screen.text())
                self.click(master, screen, "components")
                self.assertNotIn("COMPONENT MAP", screen.text())
                os.write(master, b"/im")
                self.update_screen(master, screen)
                self.click(master, screen, "/image")
                self.assertIn("actual image pixels", screen.text())
                os.write(master, b"\x0eCLICK-SEND")
                self.update_screen(master, screen)
                self.click(master, screen, "↑")
                self.assertIn("CLICK-SEND", screen.text())
                self.assertIn("Working", screen.text())
                self.click(master, screen, "■")
                self.assertNotIn("Working", screen.text())
                self.click(master, screen, "F1 Atelier")
                self.assertIn("You: ", screen.text())
                self.click(master, screen, "F4 Mission")
                self.assertIn("Codex ▾", screen.text())
                settings = json.loads(config.read_text())
                self.assertTrue(settings["mouse"])
                settings["mouse"] = False
                config.write_text(json.dumps(settings))
                output = self.update_screen(master, screen, 0.5)
                self.assertIn(b"\x1b[?1002l", output)
                self.click(master, screen, "Chat")
                self.assertIn("Chat", screen.text())
                settings["mouse"] = True
                config.write_text(json.dumps(settings))
                output = self.update_screen(master, screen, 0.5)
                self.assertIn(b"\x1b[?1002h", output)
                self.click(master, screen, "Chat")
                self.assertIn("Agent", screen.text())
                os.write(master, b"\x03")
                self.update_screen(master, screen)
                process.wait(timeout=3)
                self.assertEqual(process.returncode, 0)
                self.assertEqual(termios.tcgetattr(slave), original)
            finally:
                if process.poll() is None:
                    process.kill()
                    process.wait()
                os.close(master)
                os.close(slave)


if __name__ == "__main__":
    unittest.main()
