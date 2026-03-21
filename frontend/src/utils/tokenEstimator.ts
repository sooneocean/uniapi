// Rough token estimation without tiktoken.
// ~4 chars per token for English/Latin text, ~2 chars per token for CJK.

export function estimateTokens(text: string): number {
  if (!text) return 0;
  const cjk = (text.match(/[\u4e00-\u9fff\u3400-\u4dbf\uf900-\ufaff]/g) || []).length;
  const rest = text.length - cjk;
  return Math.ceil(rest / 4 + cjk / 2);
}

const MODEL_PRICING: Record<string, number> = {
  'gpt-4o': 2.5 / 1_000_000,
  'gpt-4o-mini': 0.15 / 1_000_000,
  'claude-sonnet-4-20250514': 3.0 / 1_000_000,
  'claude-haiku-4-20250414': 0.8 / 1_000_000,
  'gemini-2.5-pro': 1.25 / 1_000_000,
};

export function estimateCost(tokens: number, model: string): number {
  const pricePerToken = MODEL_PRICING[model] ?? 1 / 1_000_000;
  return tokens * pricePerToken;
}
