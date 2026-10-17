#!/usr/bin/env python3
"""Build the static site with inline CSS to avoid render-blocking requests."""

import argparse
import os
from pathlib import Path
import re
import shutil
import subprocess


def build(output: Path, repository_name: str = "SAPPHIR3-ROS3/Solomon",
          revision: str | None = None,
          base_url: str = "https://sapphir3-ros3.github.io/Solomon/") -> None:
    repository = Path(__file__).resolve().parents[1]
    source = repository / "site"
    if output.is_symlink() or (output / "wiki").is_symlink():
        raise ValueError("Output must not be a symlink")
    output = output.resolve()
    if output == repository or output in source.parents or output.is_relative_to(source):
        raise ValueError("Output must be outside the source website and its ancestors")
    docs = repository / "docs"
    if output == docs or output in docs.parents or output.is_relative_to(docs):
        raise ValueError("Output must not overlap documentation")
    if revision is None:
        revision = subprocess.check_output(
            ["git", "rev-parse", "HEAD"], cwd=repository, text=True).strip()
    output.mkdir(parents=True, exist_ok=True)
    shutil.copytree(source, output, dirs_exist_ok=True)

    stylesheet = re.compile(r'  <link rel="stylesheet" href="([^"]+)">\n')
    css_url = re.compile(r'url\([\"\']?([^\)\"\']+)[\"\']?\)')
    for page in source.rglob("*.html"):
        html = page.read_text(encoding="utf-8")
        styles = []

        def collect(match: re.Match) -> str:
            css_path = (page.parent / match[1]).resolve()
            if not css_path.is_relative_to(source):
                raise ValueError(f"Stylesheet outside site: {css_path}")

            def rebase(asset: re.Match) -> str:
                url = asset[1]
                if url.startswith(("data:", "https:", "http:", "#", "/")):
                    return asset[0]
                target = (css_path.parent / url).resolve()
                if not target.is_relative_to(source) or not target.is_file():
                    raise ValueError(f"Missing CSS asset: {target}")
                relative = Path(os.path.relpath(target, page.parent)).as_posix()
                return f'url("{relative}")'

            styles.append(css_url.sub(rebase, css_path.read_text(encoding="utf-8")))
            return "__INLINE_STYLES__\n" if len(styles) == 1 else ""

        html = stylesheet.sub(collect, html)
        if styles:
            html = html.replace("__INLINE_STYLES__", "  <style>\n" + "\n".join(styles) + "  </style>")
        (output / page.relative_to(source)).write_text(html, encoding="utf-8")
    environment = os.environ.copy()
    environment.update(GOPROXY="off", GOTOOLCHAIN="local", GOSUMDB="off")
    subprocess.run(
        ["go", "run", "-mod=readonly", "./scripts/site",
         "--output", str(output / "wiki"),
         "--repository", repository_name, "--revision", revision,
         "--base-url", base_url],
        cwd=repository, env=environment, check=True)
    print(f"Built website: {output}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--repository", default="SAPPHIR3-ROS3/Solomon")
    parser.add_argument("--revision")
    parser.add_argument("--base-url", default="https://sapphir3-ros3.github.io/Solomon/")
    args = parser.parse_args()
    build(args.output, args.repository, args.revision, args.base_url)
