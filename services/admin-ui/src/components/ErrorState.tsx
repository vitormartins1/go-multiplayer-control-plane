export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="error-state">
      <div>{message}</div>
      {onRetry && (
        <div style={{ marginTop: 12 }}>
          <button onClick={onRetry}>Tentar de novo</button>
        </div>
      )}
    </div>
  );
}
