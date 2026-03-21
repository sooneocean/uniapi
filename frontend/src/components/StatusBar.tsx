interface Props {
  tokensIn: number;
  tokensOut: number;
  latencyMs: number;
  streamingSpeed?: number;
}

export default function StatusBar({ tokensIn, tokensOut, latencyMs, streamingSpeed }: Props) {
  if (tokensIn === 0 && tokensOut === 0 && latencyMs === 0 && (!streamingSpeed || streamingSpeed === 0)) {
    return null;
  }

  return (
    <div className="px-4 py-1 bg-gray-800 border-t border-gray-700 text-xs text-gray-400 text-center flex items-center justify-center gap-3">
      {(tokensIn > 0 || tokensOut > 0) && (
        <span>Tokens: {tokensIn} ↑ {tokensOut} ↓</span>
      )}
      {latencyMs > 0 && (
        <span>Latency: {latencyMs}ms</span>
      )}
      {streamingSpeed !== undefined && streamingSpeed > 0 && (
        <span>⚡ {streamingSpeed} tok/s</span>
      )}
    </div>
  );
}
