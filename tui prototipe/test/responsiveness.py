import fcntl
import os
from pathlib import Path
import pty
import struct
import subprocess
import tempfile
import termios
import time
import unittest
import terminal as terminal_support
from screen import Screen

ROOT = terminal_support.ROOT


class ResponsivenessTests(unittest.TestCase):
    drain = terminal_support.TerminalTests.drain

    def test_keys_and_click_during_long_transcript_stream(self):
        with tempfile.TemporaryDirectory(prefix="solomon-stream-input-") as folder:
            master, slave = pty.openpty()
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
            env = dict(os.environ, SOLOMON_TUI_CONFIG=str(Path(folder) / "interface.json"), TERM="xterm-256color", COLORTERM="truecolor")
            process = subprocess.Popen([str(ROOT / "mission"), "--demo"], stdin=slave, stdout=slave, stderr=slave, env=env)
            screen = Screen()
            try:
                screen.feed(self.drain(master, 0.5))
                os.write(master, b"START-STREAM\r")
                screen.feed(self.drain(master, 0.4))
                started = time.monotonic()
                os.write(master, b"TYPE-NOW")
                deadline = started + 1
                while time.monotonic() < deadline:
                    screen.feed(self.drain(master, 0.02))
                    if "TYPE-NOW" in screen.text():
                        break
                elapsed = time.monotonic() - started
                print(f"Typing during stream: {elapsed:.3f}s", flush=True)
                self.assertIn("TYPE-NOW", screen.text())
                self.assertLess(elapsed, 0.5)
                x, y = screen.find("Agent")
                os.write(master, f"\x1b[<0;{x+1};{y+1}M\x1b[<0;{x+1};{y+1}m".encode())
                screen.feed(self.drain(master, 0.1))
                self.assertIn("Chat", screen.text())
                self.assertIn("Working", screen.text())
                self.assertIn("TYPE-NOW", screen.text())
                os.write(master, b"\x03")
                self.drain(master)
                process.wait(timeout=3)
            finally:
                if process.poll() is None:
                    process.kill()
                    process.wait()
                os.close(master)
                os.close(slave)

    def test_burst_does_not_delay_following_click_and_keys(self):
        with tempfile.TemporaryDirectory(prefix="solomon-tui-response-") as folder:
            master, slave = pty.openpty()
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
            env = dict(os.environ, SOLOMON_TUI_CONFIG=str(Path(folder) / "interface.json"), TERM="xterm-256color", COLORTERM="truecolor")
            process = subprocess.Popen([str(ROOT / "laboratory"), "--components"], stdin=slave, stdout=slave, stderr=slave, env=env)
            screen = Screen()
            try:
                screen.feed(self.drain(master, 0.4))
                x, y = screen.find("Transcript")
                os.write(master, f"\x1b[<0;{x+1};{y+1}M\x1b[<0;{x+1};{y+1}m".encode())
                screen.feed(self.drain(master, 0.5))
                x, y = screen.find("Composer")
                burst = b"\x1b[<65;60;15M" * 24
                click = f"\x1b[<0;{x+1};{y+1}M\x1b[<0;{x+1};{y+1}m".encode()
                started = time.monotonic()
                os.write(master, burst + click + b"\x1b[18~")
                screen.feed(self.drain(master, 0.09))
                os.write(master,b"INPUT-AFTER-BURST")
                deadline = started + 12
                while time.monotonic() < deadline:
                    screen.feed(self.drain(master, 0.03))
                    if "INPUT-AFTER-BURST" in screen.text():
                        break
                elapsed = time.monotonic() - started
                print(f"Input after scroll/click burst: {elapsed:.3f}s", flush=True)
                self.assertIn("INPUT-AFTER-BURST", screen.text())
                if os.environ.get("SOLOMON_MEASURE_ONLY") != "1":
                    self.assertLess(elapsed, 1.2, "input waited behind repeated expensive renders")
                os.write(master, b"\x03")
                self.drain(master)
                process.wait(timeout=3)
            finally:
                if process.poll() is None:
                    process.kill()
                    process.wait()
                os.close(master)
                os.close(slave)


if __name__ == "__main__":
    unittest.main()
