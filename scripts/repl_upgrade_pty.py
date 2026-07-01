#!/usr/bin/env python3
import argparse
import os
import select
import subprocess
import sys
import time


def spawn_unix(exe, env):
    import pty

    master, slave = pty.openpty()
    proc = subprocess.Popen(
        [exe],
        stdin=slave,
        stdout=slave,
        stderr=slave,
        env=env,
        close_fds=True,
    )
    os.close(slave)
    return master, proc


def spawn_windows(exe, env):
    from winpty import PtyProcess

    proc = PtyProcess.spawn(exe, env=env)
    return proc, proc


def read_unix(master, proc, deadline):
    while time.time() < deadline:
        if proc.poll() is not None:
            while True:
                try:
                    chunk = os.read(master, 4096)
                except OSError:
                    break
                if not chunk:
                    break
                sys.stdout.buffer.write(chunk)
                sys.stdout.buffer.flush()
            break
        ready, _, _ = select.select([master], [], [], 0.25)
        if not ready:
            continue
        try:
            chunk = os.read(master, 4096)
        except OSError:
            break
        if not chunk:
            break
        sys.stdout.buffer.write(chunk)
        sys.stdout.buffer.flush()


def read_windows(proc, deadline):
    while time.time() < deadline:
        try:
            chunk = proc.read(4096)
        except EOFError:
            break
        if not chunk:
            time.sleep(0.25)
            if not proc.isalive():
                break
            continue
        sys.stdout.write(chunk)
        sys.stdout.flush()


def main():
    parser = argparse.ArgumentParser(description="Send /upgrade to solomon REPL via pseudo-TTY")
    parser.add_argument("exe")
    parser.add_argument("--boot-wait", type=float, default=4.0)
    parser.add_argument("--deadline", type=float, default=120.0)
    args = parser.parse_args()

    env = os.environ.copy()
    env.setdefault("NO_COLOR", "1")

    deadline = time.time() + args.deadline

    if os.name == "nt":
        proc, pty_obj = spawn_windows(args.exe, env)
        time.sleep(args.boot_wait)
        pty_obj.write("/upgrade\r")
        read_windows(pty_obj, deadline)
        try:
            if pty_obj.isalive():
                pty_obj.close()
        except Exception:
            pass
        return 0

    master, proc = spawn_unix(args.exe, env)
    time.sleep(args.boot_wait)
    os.write(master, b"/upgrade\n")
    read_unix(master, proc, deadline)
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
