import json
import os
from pathlib import Path
import pty
import shutil
import struct
import subprocess
import tempfile
import termios
import fcntl
import time
import unittest
import terminal as terminal_support

ROOT = terminal_support.ROOT


@unittest.skipUnless(Path("/proc/self/environ").exists(), "Linux PTY process inspection")
class DevelopmentTests(unittest.TestCase):
    drain = terminal_support.TerminalTests.drain

    def frontend(self, supervisor):
        pids = set()
        try:
            for children in Path(f"/proc/{supervisor.pid}/task").glob("*/children"):
                try:
                    pids.update(children.read_text().split())
                except OSError:
                    pass
        except OSError:
            return None
        for pid in pids:
            try:
                fields = Path(f"/proc/{pid}/environ").read_bytes().split(b"\0")
            except OSError:
                continue
            paths = {}
            for field in fields:
                if field.startswith((b"SOLOMON_TUI_STATE_PATH=", b"SOLOMON_TUI_DEV_STATUS=")):
                    key, value = field.decode().split("=", 1)
                    paths[key] = value
            if "SOLOMON_TUI_STATE_PATH" in paths:
                return int(pid), paths
        return None

    def wait_for(self, master, supervisor, predicate, timeout=25):
        end = time.monotonic() + timeout
        output = b""
        while time.monotonic() < end:
            output += self.drain(master, 0.15)
            if supervisor.poll() is not None:
                self.fail(f"Supervisor exited {supervisor.returncode}: {output[-1500:]!r}")
            frontend = self.frontend(supervisor)
            if frontend and predicate(frontend):
                return frontend, output
        self.fail(f"Timed out waiting for reload: {output[-1500:]!r}")

    def test_restore_state_before_new_text_default(self):
        with tempfile.TemporaryDirectory(prefix="solomon-tui-migration-") as temporary:
            folder=Path(temporary)
            config=folder/"interface.json"
            env=dict(os.environ,SOLOMON_TUI_CONFIG=str(config),TERM="xterm-256color",COLORTERM="truecolor")
            subprocess.run([str(ROOT/"laboratory"),"--init-config"],env=env,capture_output=True,check=True)
            old_config=json.loads(config.read_text())
            old_config["text"].pop("tool_icon")
            state=folder/"state.json"
            tool={"name":"orchestrate","arguments":{"intent":"Restored tool call","source":"package main\nfunc main() {}\n"},"result":"done"}
            state.write_text(json.dumps({"version":1,"reload_token":"old-session","data":{"Variant":4,"Width":120,"Height":40,"Selected":0,"Draft":"PRESERVED-DRAFT","Details":True,"Config":old_config,"Messages":[{"Role":"TOOLCALL","Text":json.dumps(tool)}]}}))
            env["SOLOMON_TUI_STATE_PATH"]=str(state)
            master,slave=pty.openpty()
            fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack("HHHH",40,120,0,0))
            process=subprocess.Popen([str(ROOT/"laboratory")],stdin=slave,stdout=slave,stderr=slave,env=env)
            try:
                output=self.drain(master,0.5)
                self.assertIsNone(process.poll(),"new frontend rejected an older session")
                self.assertIn("●".encode(),output)
                self.assertIn(b"Restored tool call",output)
                self.assertIn(b"PRESERVED-DRAFT",output)
                os.write(master,b"\x03")
                self.drain(master)
                process.wait(timeout=3)
                self.assertEqual(process.returncode,0)
            finally:
                if process.poll() is None:
                    process.kill()
                    process.wait()
                os.close(master)
                os.close(slave)

    def test_rebuild_handoff_preserves_state_and_failed_build(self):
        with tempfile.TemporaryDirectory(prefix="solomon-tui-hmr-") as temporary:
            sandbox = Path(temporary)
            frontend_dir = sandbox / "frontend"
            frontend_dir.mkdir()
            for item in ROOT.iterdir():
                if item.suffix == ".go" or item.name in ("go.mod", "go.sum"):
                    shutil.copy2(item, frontend_dir / item.name)
            shutil.copytree(ROOT / "fixtures", frontend_dir / "fixtures")
            shutil.copytree(ROOT.parent / "tui", sandbox / "tui", ignore=shutil.ignore_patterns("test"))
            config = sandbox / "interface.json"
            env = dict(os.environ, SOLOMON_TUI_CONFIG=str(config), TERM="xterm-256color", COLORTERM="truecolor")
            master, slave = pty.openpty()
            original = termios.tcgetattr(slave)
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 40, 120, 0, 0))
            supervisor = subprocess.Popen([str(ROOT / "tui-dev"), "4"], cwd=frontend_dir, stdin=slave, stdout=slave, stderr=slave, env=env)
            try:
                (first_pid, paths), _ = self.wait_for(master, supervisor, lambda child: True)
                self.drain(master, 0.5)
                os.write(master, b"/artifacts\r")
                self.drain(master, 0.4)
                os.write(master, b"\x1b[6~DRAFT-PERSIST\t")
                self.drain(master, 0.3)
                changed = frontend_dir / "generation.go"
                changed.write_text('package main\nfunc init() { previewModels[0] = "Rebuilt model" }\n')
                (second_pid, _), output = self.wait_for(master, supervisor, lambda child: child[0] != first_pid)
                output += self.drain(master, 0.6)
                self.assertIn(b"DRAFT-PERSIST", output)
                self.assertIn(b"Rebuilt model", output)
                snapshot = json.loads(Path(paths["SOLOMON_TUI_STATE_PATH"]).read_text())["data"]
                self.assertEqual(snapshot["Draft"], "DRAFT-PERSIST")
                self.assertEqual(snapshot["Selected"], 1)
                self.assertGreater(snapshot["Scroll"], 0)
                self.assertGreater(len(snapshot["Messages"]), 10)
                changed.write_text("package main\nfunc {\n")
                status_path = Path(paths["SOLOMON_TUI_DEV_STATUS"])

                def failed(child):
                    try:
                        return json.loads(status_path.read_text())["phase"] == "error"
                    except (OSError, json.JSONDecodeError):
                        return False

                (still_running, _), output = self.wait_for(master, supervisor, failed)
                self.assertEqual(still_running, second_pid)
                os.write(master, b"Y")
                output = self.drain(master, 0.4)
                self.assertIn(b"DRAFT-PERSISTY", output)
                changed.write_text('package main\nfunc init() { previewModels[0] = "Recovered model" }\n')
                _, output = self.wait_for(master, supervisor, lambda child: child[0] != second_pid)
                output += self.drain(master, 0.5)
                self.assertIn(b"Recovered model", output)
                self.assertIn(b"DRAFT-PERSISTY", output)
                os.write(master, b"\x11")
                self.drain(master, 0.5)
                supervisor.wait(timeout=5)
                self.assertEqual(supervisor.returncode, 0)
                self.assertEqual(termios.tcgetattr(slave), original)
            finally:
                if supervisor.poll() is None:
                    supervisor.terminate()
                    try:
                        supervisor.wait(timeout=4)
                    except subprocess.TimeoutExpired:
                        supervisor.kill()
                        supervisor.wait()
                os.close(master)
                os.close(slave)


if __name__ == "__main__":
    unittest.main()
