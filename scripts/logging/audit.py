import collections
import json
from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
EXTENSIONS = {".go", ".ts", ".tsx", ".js", ".mjs", ".py", ".sh", ".ps1"}
EXCLUDED = {"node_modules", "vendor", "dist", "build", "assets", "wailsjs", ".git", ".venv", "__pycache__"}
PATTERNS = {
    "error_handling": re.compile(r"\bif\s+(?:\w+\s*:=.*;\s*)?err\s*!=\s*nil|\bcatch\b|\bexcept\b"),
    "logging": re.compile(r"\blogging\.|\blog\.(?:Print|Fatal)|\bconsole\.(?:error|warn|log)|\bslog\."),
    "processes": re.compile(r"exec\.Command(?:Context)?\(|\b(?:spawn|execFile|subprocess\.run)\("),
    "ignored_results": re.compile(r"_\s*=\s*.*\.(?:Write|Save|Encode|Run|Wait|Close|Remove|Rename)\w*\("),
    "silent_catch": re.compile(r"catch\s*\([^)]*\)\s*\{\s*\}|catch\s*\{\s*\}|\.catch\(\(\)\s*=>\s*\{\s*\}\)"),
}


def sources():
    result = subprocess.run(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
        cwd=ROOT, check=True, capture_output=True,
    )
    for name in sorted(set(result.stdout.decode().split("\0"))):
        path = ROOT / name
        if not name or not path.is_file() or path.suffix not in EXTENSIONS:
            continue
        if any(part in EXCLUDED for part in path.relative_to(ROOT).parts):
            continue
        if any(part in {"test", "tests", "e2e"} for part in path.parts) or path.name.endswith("_test.go"):
            continue
        yield path


def audit():
    groups = collections.defaultdict(collections.Counter)
    candidates = []
    total = 0
    for path in sources():
        relative = path.relative_to(ROOT)
        text = path.read_text(errors="replace")
        if "Code generated" in text[:1000]:
            continue
        total += 1
        group = "/".join(relative.parts[:2]) if len(relative.parts) > 2 else relative.parts[0]
        flags = {name: bool(pattern.search(text)) for name, pattern in PATTERNS.items()}
        groups[group]["files"] += 1
        groups[group].update(name for name, present in flags.items() if present)
        reasons = []
        if not flags["logging"] and flags["processes"]:
            reasons.append("process_without_local_logging")
        if flags["ignored_results"]:
            reasons.append("ignored_result_requires_review")
        if flags["silent_catch"]:
            reasons.append("silent_catch_requires_review")
        if reasons:
            candidates.append({"path": str(relative), "reasons": reasons})
    return {
        "files_scanned": total,
        "scope": "repository-owned runtime code and scripts, excluding tests and generated or bundled assets",
        "interpretation": "Heuristic review inventory, not proof of a defect. Errors may be observed by callers, HTTP middleware, tool dispatch, or process helpers. Expected probes and cleanup may intentionally remain quiet.",
        "groups": dict(sorted(groups.items())),
        "review_candidates": candidates,
    }


if __name__ == "__main__":
    print(json.dumps(audit(), indent=2))
