import argparse
import re
import shutil
from pathlib import Path
from urllib.parse import quote, unquote, urlsplit


def page(path):
    return 'Home' if path.as_posix() == 'README.md' else '--'.join(path.with_suffix('').parts)


def rewrite(text, source, docs, repository, revision):
    def convert(match):
        raw = match[2]
        url = urlsplit(raw)
        if url.scheme or url.netloc or not url.path or url.path.startswith('/'):
            return match[0]
        target = (source.parent / unquote(url.path)).resolve()
        suffix = ('?' + url.query if url.query else '') + ('#' + url.fragment if url.fragment else '')
        try:
            relative = target.relative_to(docs)
            dest = page(relative) if relative.suffix.lower() == '.md' else relative.as_posix()
            dest = quote(dest) + suffix
        except ValueError:
            relative = target.relative_to(docs.parent)
            kind = 'tree' if target.is_dir() else 'blob'
            dest = f'https://github.com/{repository}/{kind}/{revision}/{quote(relative.as_posix())}{suffix}'
        return match[1] + dest + match[3]

    patterns = [r'(!?\[[^\]\n]*\]\()([^\s)]+)(\))', r'(?m)^(\s*\[[^\]\n]+\]:\s*)(\S+)(.*)$', r'((?:href|src)=")([^"]+)(")']
    result = []
    fence = None
    for line in text.splitlines(keepends=True):
        marker = re.match(r'^\s{0,3}(`{3,}|~{3,})', line)
        if marker:
            token = marker[1]
            if fence is None:
                fence = token
            elif token[0] == fence[0] and len(token) >= len(fence):
                fence = None
        elif fence is None:
            for pattern in patterns:
                line = re.sub(pattern, convert, line)
        result.append(line)
    return ''.join(result)


def mirror(docs, output, repository, revision):
    docs, output = docs.resolve(), output.resolve()
    if docs == output or docs in output.parents or output in docs.parents:
        raise ValueError('source and output must not overlap')
    if not (docs / 'README.md').is_file():
        raise ValueError('docs/README.md is required')
    sources = sorted(p for p in docs.rglob('*') if p.is_file())
    rendered = {}
    names = set()
    for source in sources:
        if source.is_symlink():
            raise ValueError(f'symlink not supported: {source}')
        relative = source.relative_to(docs)
        name = page(relative) + '.md' if relative.suffix.lower() == '.md' else relative.as_posix()
        if name.casefold() in names:
            raise ValueError(f'wiki page collision: {name}')
        names.add(name.casefold())
        rendered[name] = rewrite(source.read_text(encoding='utf-8'), source, docs, repository, revision).encode('utf-8') if relative.suffix.lower() == '.md' else source.read_bytes()
    output.mkdir(parents=True, exist_ok=True)
    for child in output.iterdir():
        if child.name == '.git':
            continue
        if child.is_dir() and not child.is_symlink():
            shutil.rmtree(child)
        else:
            child.unlink()
    for name, content in rendered.items():
        target = output / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(content)
    return len(rendered)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--docs', type=Path, default=Path('docs'))
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--repository', required=True)
    parser.add_argument('--revision', default='main')
    args = parser.parse_args()
    print('Files mirrored:', mirror(args.docs, args.output, args.repository, args.revision))


if __name__ == '__main__':
    main()
