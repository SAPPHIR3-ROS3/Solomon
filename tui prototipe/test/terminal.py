import fcntl
import os
import pty
import re
import select
import signal
import struct
import subprocess
import termios
import time
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
TEST_CONFIGURATION = tempfile.TemporaryDirectory(prefix="solomon-tui-tests-")
os.environ["SOLOMON_TUI_CONFIG"] = str(Path(TEST_CONFIGURATION.name) / "interface.json")


class TerminalTests(unittest.TestCase):
    def drain(self, master, duration=0.2):
        data = b""
        end = time.monotonic() + duration
        while time.monotonic() < end:
            ready, _, _ = select.select([master], [], [], 0.025)
            if ready:
                try:
                    data += os.read(master, 65536)
                except OSError:
                    break
        return data

    def scroll_position(self, output):
        plain = re.sub(rb"\x1b\[[0-9;]*m", b"", output)
        matches = re.findall(rb"\x1b\]777;scroll;(\d+);(\d+);(\d+)\x1b\\", plain)
        self.assertTrue(matches, f"Missing scroll position: {plain[-200:]!r}")
        return tuple(map(int, matches[-1]))

    def test_scroll_wheel_and_paging(self):
        for name in ("atelier", "orbit", "paper", "mission"):
            for dataset in ("--demo", "--artifacts"):
                with self.subTest(layout=name, dataset=dataset):
                    master, slave = pty.openpty()
                    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
                    env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor", SOLOMON_TUI_SCROLL_TRACE="1")
                    process = subprocess.Popen([str(ROOT / name), dataset], stdin=slave, stdout=slave, stderr=slave, env=env)
                    try:
                        first = self.scroll_position(self.drain(master, 0.5))
                        self.assertEqual(first[0], 1)
                        os.write(master, b"\x1b[<65;30;10M")
                        down = self.scroll_position(self.drain(master))
                        self.assertEqual(down[0], 4)
                        os.write(master, b"\x1b[6~")
                        page = self.scroll_position(self.drain(master))
                        self.assertGreater(page[0], down[0])
                        os.write(master, b"\x1b[5~")
                        back = self.scroll_position(self.drain(master))
                        self.assertEqual(back[0], down[0])
                        os.write(master, b"\x1b[H")
                        self.assertEqual(self.scroll_position(self.drain(master))[0], 1)
                        os.write(master, b"\x1b[<64;30;10M" * 12)
                        self.drain(master)
                        os.write(master, b"\x1b[<65;110;38M")
                        self.assertEqual(self.scroll_position(self.drain(master))[0], 4)
                        os.write(master, b"\x1b[F")
                        end = self.scroll_position(self.drain(master))
                        self.assertEqual(end[1], end[2])
                        os.write(master, b"\x1b[<65;30;10M" * 12)
                        self.drain(master)
                        os.write(master, b"\x1b[<64;30;10M")
                        up = self.scroll_position(self.drain(master))
                        self.assertEqual(up[0], end[0] - 3)
                        os.write(master, b"\x1b[H")
                        self.drain(master)
                        os.write(master, b"\x1b[6~")
                        self.drain(master)
                        os.write(master, b"\x1bOS")
                        self.drain(master)
                        os.write(master, b"\x1b[<64;30;10M")
                        switched = self.scroll_position(self.drain(master))
                        self.assertLess(switched[1], switched[2])
                        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 32, 100, 0, 0))
                        process.send_signal(signal.SIGWINCH)
                        self.drain(master)
                        os.write(master, b"\x1b[H")
                        self.drain(master)
                        os.write(master, b"\x1b[<65;30;10M")
                        self.assertEqual(self.scroll_position(self.drain(master))[0], 4)
                        os.write(master, b"\x03")
                        self.drain(master)
                        process.wait(timeout=3)
                        self.assertEqual(process.returncode, 0)
                    finally:
                        if process.poll() is None:
                            process.kill()
                            process.wait()
                        os.close(master)
                        os.close(slave)

    def test_slash_and_fragmented_mouse(self):
        for name in ("atelier", "orbit", "paper", "mission"):
            with self.subTest(layout=name):
                master, slave = pty.openpty()
                fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
                env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor", SOLOMON_TUI_SCROLL_TRACE="1")
                process = subprocess.Popen([str(ROOT / name), "--demo"], stdin=slave, stdout=slave, stderr=slave, env=env)
                try:
                    self.drain(master, 0.6)
                    for part in (b"\x1b[", b"<65;", b"30;10", b"M"):
                        os.write(master, part)
                        time.sleep(0.008)
                    output = self.drain(master)
                    self.assertEqual(self.scroll_position(output)[0], 4)
                    os.write(master, b"\x1b[M" + bytes((97, 62, 42)))
                    self.assertEqual(self.scroll_position(self.drain(master))[0], 7)
                    os.write(master, b"\x0e")
                    self.drain(master)
                    os.write(master, b"/im")
                    output = self.drain(master)
                    self.assertIn(b"/image", output)
                    self.assertIn(b"Tab complete", output)
                    os.write(master, b"\t\r")
                    output = self.drain(master, 0.4)
                    self.assertIn(b"actual image pixels", output)
                    self.assertGreater(output.count("▀".encode()), 100)
                    os.write(master, b"\x0e")
                    self.drain(master)
                    os.write(master, b"P")
                    self.drain(master)
                    for part in (b"\x1b", b"[<", b"65;110;", b"38M"):
                        os.write(master, part)
                        time.sleep(0.008)
                    os.write(master, b"\x1b[<0;30;10M\x1b[<0;30;10m")
                    os.write(master, b"robe\r")
                    output = self.drain(master, 0.4)
                    plain = re.sub(rb"\x1b\[[0-9;]*m", b"", output).decode(errors="replace")
                    self.assertRegex(plain, r"│ Probe +│")
                    os.write(master, b"\x03")
                    self.drain(master)
                    process.wait(timeout=3)
                    self.assertEqual(process.returncode, 0)
                finally:
                    if process.poll() is None:
                        process.kill()
                        process.wait()
                    os.close(master)
                    os.close(slave)

    def test_interaction_and_terminal_restore(self):
        for name in ("atelier", "orbit", "paper", "mission"):
            with self.subTest(layout=name):
                master, slave = pty.openpty()
                original = termios.tcgetattr(slave)
                fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
                env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor", SOLOMON_TUI_SCROLL_TRACE="1")
                process = subprocess.Popen([str(ROOT / name)], stdin=slave, stdout=slave, stderr=slave, env=env)
                try:
                    output = self.drain(master, 0.5)
                    self.assertIn(b"\x1b[?1049h", output)
                    self.assertIn(b"new thread", output)
                    os.write(master, "Test Unicode: caffè 世界 🚀".encode())
                    self.drain(master)
                    os.write(master, b"\r")
                    output = self.drain(master, 0.5)
                    self.assertIn(b"simulated", output)
                    os.write(master, b"\x1bOS")
                    self.drain(master)
                    os.write(master, b"\x0b")
                    output = self.drain(master)
                    self.assertIn(b"COMMAND CENTER", output)
                    os.write(master, b"\x1b")
                    self.drain(master)
                    os.write(master, b"\x0e")
                    output = self.drain(master)
                    self.assertIn(b"new thread", output)
                    os.write(master, b"/demo\r")
                    output = self.drain(master)
                    self.assertIn(b"searchTools", output)
                    os.write(master, b"\x1b[17~")
                    output = self.drain(master)
                    self.assertIn(b"Artifact gallery", output)
                    os.write(master, b"\x1b[F")
                    output = self.drain(master)
                    self.assertIn(b"gallery is complete", output)
                    os.write(master, b"\x1b[H")
                    self.drain(master)
                    os.write(master, b"/new\r")
                    output = self.drain(master)
                    self.assertIn(b"new thread", output)
                    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 80, 0, 0))
                    process.send_signal(signal.SIGWINCH)
                    self.drain(master)
                    os.write(master, b"\x03")
                    output = self.drain(master)
                    process.wait(timeout=3)
                    self.assertEqual(process.returncode, 0)
                    self.assertIn(b"\x1b[?1049l", output)
                    self.assertEqual(termios.tcgetattr(slave), original)
                finally:
                    if process.poll() is None:
                        process.kill()
                        process.wait()
                    os.close(master)
                    os.close(slave)


if __name__ == "__main__":
    unittest.main()
