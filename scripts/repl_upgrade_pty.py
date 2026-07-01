#!/usr/bin/env python3
import argparse
import os
import queue
import re
import select
import subprocess
import sys
import threading
import time

ANSI_RE = re.compile(r"\x1b\[[0-9;]*[A-Za-z]")


def strip_ansi(text):
    return ANSI_RE.sub("", text)


def has_prompt(buf):
    return "You:" in strip_ansi(buf)


def write_output(data):
    if isinstance(data, str):
        data = data.encode("utf-8", errors="replace")
    sys.stdout.buffer.write(data)
    sys.stdout.buffer.flush()


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
    out_q = queue.Queue()

    def reader():
        while proc.isalive() or not proc.eof:
            try:
                chunk = proc.read(4096)
            except EOFError:
                break
            if chunk:
                out_q.put(chunk)

    threading.Thread(target=reader, daemon=True).start()
    return proc, out_q


def read_chunk_windows(out_q, timeout=0.25):
    try:
        return out_q.get(timeout=timeout)
    except queue.Empty:
        return ""


def read_chunk_unix(master):
    try:
        return os.read(master, 4096)
    except OSError:
        return b""


def wait_for_prompt_unix(master, proc, deadline):
    buf = ""
    while time.time() < deadline:
        if proc.poll() is not None:
            chunk = read_chunk_unix(master)
            if chunk:
                write_output(chunk)
            return False
        ready, _, _ = select.select([master], [], [], 0.25)
        if not ready:
            continue
        chunk = read_chunk_unix(master)
        if not chunk:
            continue
        write_output(chunk)
        buf += chunk.decode("utf-8", errors="replace")
        if has_prompt(buf):
            time.sleep(1.0)
            return True
    return False


def wait_for_prompt_windows(proc, out_q, deadline):
    buf = ""
    while time.time() < deadline:
        chunk = read_chunk_windows(out_q)
        if not chunk:
            if not proc.isalive():
                return False
            continue
        write_output(chunk)
        buf += chunk
        if has_prompt(buf):
            time.sleep(1.0)
            return True
    return False


def read_until_deadline_unix(master, proc, deadline):
    while time.time() < deadline:
        if proc.poll() is not None:
            while True:
                chunk = read_chunk_unix(master)
                if not chunk:
                    break
                write_output(chunk)
            break
        ready, _, _ = select.select([master], [], [], 0.25)
        if not ready:
            continue
        chunk = read_chunk_unix(master)
        if not chunk:
            break
        write_output(chunk)


def read_until_deadline_windows(proc, out_q, deadline):
    while time.time() < deadline:
        chunk = read_chunk_windows(out_q)
        if not chunk:
            if not proc.isalive():
                break
            continue
        write_output(chunk)


def send_submit_unix(master, submit_only):
    time.sleep(0.25)
    if submit_only:
        os.write(master, b"\n")
        return
    os.write(master, b"/upgrade\n")


def send_submit_windows(proc, submit_only):
    time.sleep(0.25)
    if submit_only:
        proc.write("\r")
        return
    proc.write("/upgrade\r")


def main():
    parser = argparse.ArgumentParser(description="Send /upgrade to solomon REPL via pseudo-TTY")
    parser.add_argument("exe")
    parser.add_argument("--prompt-wait", type=float, default=90.0)
    parser.add_argument("--deadline", type=float, default=120.0)
    parser.add_argument(
        "--submit-only",
        action="store_true",
        help="only press Enter; requires SOLOMON_REPL_PREFILL=/upgrade",
    )
    args = parser.parse_args()

    env = os.environ.copy()
    env.setdefault("NO_COLOR", "1")
    submit_only = args.submit_only or bool(env.get("SOLOMON_REPL_PREFILL"))

    prompt_deadline = time.time() + args.prompt_wait
    end_deadline = prompt_deadline + args.deadline

    if os.name == "nt":
        proc, out_q = spawn_windows(args.exe, env)
        if not wait_for_prompt_windows(proc, out_q, prompt_deadline):
            print("repl upgrade smoke: REPL prompt not ready", file=sys.stderr)
            return 1
        send_submit_windows(proc, submit_only)
        read_until_deadline_windows(proc, out_q, end_deadline)
        try:
            if proc.isalive():
                proc.close()
        except Exception:
            pass
        return 0

    master, proc = spawn_unix(args.exe, env)
    if not wait_for_prompt_unix(master, proc, prompt_deadline):
        print("repl upgrade smoke: REPL prompt not ready", file=sys.stderr)
        return 1
    send_submit_unix(master, submit_only)
    read_until_deadline_unix(master, proc, end_deadline)
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
