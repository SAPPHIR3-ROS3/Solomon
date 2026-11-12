import fcntl
import os
import pty
import struct
import subprocess
import termios
import unittest
import terminal as terminal_support

ROOT = terminal_support.ROOT


class HomeTests(unittest.TestCase):
    drain = terminal_support.TerminalTests.drain

    def start(self, name):
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
        original = termios.tcgetattr(slave)
        env = dict(os.environ, TERM="xterm-256color", COLORTERM="truecolor", SOLOMON_PREVIEW_NAME="Test User")
        process = subprocess.Popen([str(ROOT / name)], stdin=slave, stdout=slave, stderr=slave, env=env)
        self.drain(master, 0.4)
        return master, slave, original, process

    def stop(self, master, slave, process):
        if process.poll() is None:
            os.write(master, b"\x03")
            self.drain(master)
            process.wait(timeout=3)
        os.close(master)
        os.close(slave)

    def test_image_command_interrupts_streaming(self):
        for name in ("orbit", "mission"):
            with self.subTest(layout=name):
                master, slave, original, process = self.start(name)
                try:
                    os.write(master, b"Start a response\r")
                    self.drain(master, 0.12)
                    os.write(master, b"/image\r")
                    output = self.drain(master, 0.5)
                    self.assertIn(b"actual image pixels", output)
                    self.assertGreater(output.count("▀".encode()), 100)
                    os.write(master, b"\x03")
                    self.drain(master)
                    process.wait(timeout=3)
                    self.assertEqual(process.returncode, 0)
                    self.assertEqual(termios.tcgetattr(slave), original)
                finally:
                    self.stop(master, slave, process)

    def test_mission_gui_controls(self):
        master, slave, original, process = self.start("mission")
        try:
            os.write(master, b"\x10")
            output = self.drain(master)
            self.assertIn(b"Select model", output)
            os.write(master, b"\x1b[B\r")
            output = self.drain(master)
            self.assertIn(b"Claude Sonnet", output)
            os.write(master, b"\x12")
            output = self.drain(master)
            self.assertIn(b"Reasoning effort", output)
            os.write(master, b"\x1b[B\r")
            output = self.drain(master)
            self.assertIn(b"Low", output)
            os.write(master, b"\t")
            output = self.drain(master)
            self.assertIn(b"Chat", output)
            os.write(master, b"Test the docked composer\r")
            output = self.drain(master, 0.3)
            self.assertIn(b"Test the docked composer", output)
            os.write(master, b"/image\r")
            output = self.drain(master, 0.4)
            self.assertIn(b"actual image pixels", output)
            os.write(master, b"\x03")
            self.drain(master)
            process.wait(timeout=3)
            self.assertEqual(termios.tcgetattr(slave), original)
        finally:
            self.stop(master, slave, process)


if __name__ == "__main__":
    unittest.main()
