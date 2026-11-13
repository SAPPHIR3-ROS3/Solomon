import asciiBanner from "../../../internal/logo/logo.txt?raw";
import asciiColors from "../../../internal/logo/colors.txt?raw";

const asciiColorRows = asciiColors.trim().split(/\r?\n/).map((row) => row.trim().split(/\s+/));

export function AsciiBanner() {
  const lines = asciiBanner.trimEnd().split(/\r?\n/);
  return (
    <pre className="welcome-ascii-banner" aria-hidden="true">
      {lines.map((line, rowIndex) => {
        const colors = asciiColorRows[rowIndex] ?? [];
        const width = Math.max(line.length, colors.length);
        const padded = line.padEnd(width, " ");
        return (
          <span className="welcome-ascii-row" key={rowIndex}>
            {Array.from(padded).map((character, columnIndex) => (
              <span
                className="welcome-ascii-cell"
                key={columnIndex}
                style={{ color: logoColor(colors[columnIndex] ?? "000000") }}
              >
                {character === " " ? "\u00a0" : character}
              </span>
            ))}
          </span>
        );
      })}
    </pre>
  );
}

function logoColor(hex: string) {
  const channels = [0, 2, 4].map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16) / 255);
  const [red, green, blue] = channels;
  const max = Math.max(red, green, blue);
  const min = Math.min(red, green, blue);
  let lightness = (max + min) / 2;
  let hue = 0;
  let saturation = 0;
  if (max !== min) {
    const delta = max - min;
    saturation = delta / (lightness > 0.5 ? 2 - max - min : max + min);
    if (max === red) hue = ((green - blue) / delta + (blue > green ? 6 : 0)) / 6;
    else if (max === green) hue = ((blue - red) / delta + 2) / 6;
    else hue = ((red - green) / delta + 4) / 6;
  }
  saturation = Math.min(1, saturation * 1.38);
  if (saturation >= 0.05 && hue >= 0.065 && hue <= 0.23) {
    saturation = Math.min(1, saturation * 1.48);
    lightness = Math.min(0.94, lightness + (1 - lightness) * 0.11);
  }
  const hueChannel = (p: number, q: number, t: number) => {
    if (t < 0) t += 1;
    if (t > 1) t -= 1;
    if (t < 1 / 6) return p + (q - p) * 6 * t;
    if (t < 1 / 2) return q;
    if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6;
    return p;
  };
  const q = lightness < 0.5 ? lightness * (1 + saturation) : lightness + saturation - lightness * saturation;
  const p = 2 * lightness - q;
  const rgb = saturation === 0
    ? [lightness, lightness, lightness]
    : [hueChannel(p, q, hue + 1 / 3), hueChannel(p, q, hue), hueChannel(p, q, hue - 1 / 3)];
  const enhanced = rgb.map((channel) => Math.max(0, Math.min(255, Math.round(128 + 1.14 * (Math.round(channel * 255) - 128)))));
  return `rgb(${enhanced.join(" ")})`;
}
