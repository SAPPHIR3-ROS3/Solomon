import codecs
import re
import unicodedata


def cell_width(text):
    return sum(0 if unicodedata.combining(char) or char in ("\ufe0e","\ufe0f") else 2 if unicodedata.east_asian_width(char) in ("W", "F") else 1 for char in text)


class Screen:
    def __init__(self, columns=120, rows=40):
        self.columns = columns
        self.rows = rows
        self.cells = [[" "] * columns for _ in range(rows)]
        self.x = self.y = 0
        self.pending = ""
        self.decoder = codecs.getincrementaldecoder("utf-8")("replace")

    def feed(self, data):
        self.pending += self.decoder.decode(data)
        while self.pending:
            if self.pending.startswith("\x1b["):
                match = re.match(r"\x1b\[([0-?]*)([ -/]*)([@-~])", self.pending)
                if not match:
                    return
                self.control(match.group(1), match.group(3))
                self.pending = self.pending[match.end():]
                continue
            if self.pending.startswith("\x1b]"):
                end = self.pending.find("\x1b\\")
                bell = self.pending.find("\a")
                if bell >= 0 and (end < 0 or bell < end):
                    self.pending = self.pending[bell + 1:]
                elif end >= 0:
                    self.pending = self.pending[end + 2:]
                else:
                    return
                continue
            char = self.pending[0]
            if char == "\x1b":
                if len(self.pending) < 2:
                    return
                self.pending = self.pending[2:]
                continue
            self.pending = self.pending[1:]
            if char == "\r":
                self.x = 0
            elif char == "\n":
                self.y = min(self.rows - 1, self.y + 1)
            elif char == "\b":
                self.x = max(0, self.x - 1)
            elif ord(char) >= 32:
                width = cell_width(char)
                if width == 0:
                    continue
                if self.x >= self.columns:
                    self.x = 0
                    self.y = min(self.rows - 1, self.y + 1)
                self.cells[self.y][self.x] = char
                if width == 2 and self.x + 1 < self.columns:
                    self.cells[self.y][self.x + 1] = ""
                self.x += width

    def control(self, parameters, command):
        if parameters.startswith("?"):
            return
        values = [int(part or "0") for part in parameters.split(";")] if parameters else []
        value = values[0] if values else 0
        count = value or 1
        if command in ("H", "f"):
            self.y = max(0, min(self.rows - 1, count - 1))
            self.x = max(0, min(self.columns - 1, (values[1] or 1) - 1 if len(values) > 1 else 0))
        elif command == "A":
            self.y = max(0, self.y - count)
        elif command == "B":
            self.y = min(self.rows - 1, self.y + count)
        elif command == "C":
            self.x = min(self.columns - 1, self.x + count)
        elif command == "D":
            self.x = max(0, self.x - count)
        elif command == "G":
            self.x = min(self.columns - 1, count - 1)
        elif command == "J" and value in (2, 3):
            self.cells = [[" "] * self.columns for _ in range(self.rows)]
        elif command == "K":
            start = 0 if value in (1, 2) else min(self.x, self.columns)
            end = min(self.x + 1, self.columns) if value == 1 else self.columns
            self.cells[self.y][start:end] = [" "] * (end - start)

    def text(self):
        return "\n".join("".join(row) for row in self.cells)

    def find(self, label):
        for y, row in enumerate(self.cells):
            text = "".join(row)
            offset = text.find(label)
            if offset >= 0:
                return cell_width(text[:offset]), y
        raise AssertionError(f"Label {label!r} not visible:\n{self.text()}")
