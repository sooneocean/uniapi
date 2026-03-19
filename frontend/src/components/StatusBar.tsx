interface Props {
  tokensIn: number;
  tokensOut: number;
  latencyMs: number;
}

export default function StatusBar({ tokensIn, tokensOut, latencyMs }: Props) {
  if (tokensIn === 0 && tokensOut === 0 && latencyMs === 0) {
    return null;
  }

  return (
    <div className="px-4 py-1 bg-gray-800 border-t border-gray-700 text-xs text-gray-400 text-center">
      Tokens: {tokensIn} ↑ {tokensOut} ↓ | Latency: {latencyMs}ms
    </div>
  );
}
