const DECIMAL_PATTERN = /^-?\d+(?:\.\d+)?$/;

interface FixedDecimal {
  value: bigint;
  scale: number;
}

function parseDecimal(input: string): FixedDecimal {
  const normalized = input.trim();
  if (!DECIMAL_PATTERN.test(normalized)) throw new Error('Enter a valid decimal value.');
  const negative = normalized.startsWith('-');
  const unsigned = negative ? normalized.slice(1) : normalized;
  const [whole, fraction = ''] = unsigned.split('.');
  return { value: BigInt(`${whole}${fraction}`) * (negative ? -1n : 1n), scale: fraction.length };
}

function align(a: FixedDecimal, b: FixedDecimal): [bigint, bigint, number] {
  const scale = Math.max(a.scale, b.scale);
  return [a.value * 10n ** BigInt(scale - a.scale), b.value * 10n ** BigInt(scale - b.scale), scale];
}

function render(value: bigint, scale: number): string {
  const negative = value < 0n;
  const digits = (negative ? -value : value).toString().padStart(scale + 1, '0');
  const whole = scale ? digits.slice(0, -scale) : digits;
  const fraction = scale ? digits.slice(-scale).replace(/0+$/, '') : '';
  return `${negative ? '-' : ''}${whole}${fraction ? `.${fraction}` : ''}`;
}

export function multiplyDecimal(a: string, b: string): string {
  const left = parseDecimal(a);
  const right = parseDecimal(b);
  return render(left.value * right.value, left.scale + right.scale);
}

export function compareDecimal(a: string, b: string): number {
  const [left, right] = align(parseDecimal(a), parseDecimal(b));
  return left < right ? -1 : left > right ? 1 : 0;
}

export function isPositiveDecimal(value: string): boolean {
  try {
    return parseDecimal(value).value > 0n;
  } catch {
    return false;
  }
}

export function formatDecimal(value: string, maximumFractionDigits = 4): string {
  const parsed = parseDecimal(value);
  const rendered = render(parsed.value, parsed.scale);
  const [whole, fraction] = rendered.split('.');
  const grouped = Number(whole).toLocaleString('en-US', { useGrouping: true, maximumFractionDigits: 0 });
  return `${grouped}${fraction ? `.${fraction.slice(0, maximumFractionDigits)}` : ''}`;
}
