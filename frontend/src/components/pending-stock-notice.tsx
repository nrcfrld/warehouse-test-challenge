import { useStockAttempt } from "@/hooks/use-stock-attempt";
import { Button } from "./ui/button";

export function PendingStockNotice({
  itemID,
  onReceive,
  onTransfer,
}: {
  itemID: string;
  onReceive: () => void;
  onTransfer: () => void;
}) {
  const receipt = useStockAttempt("receive", itemID);
  const transfer = useStockAttempt("transfer", itemID);
  if (!receipt.attempt && !transfer.attempt) return null;
  return (
    <div
      role="status"
      className="mb-6 flex flex-wrap items-center justify-between gap-4 rounded-xl border border-amber-200 bg-amber-50 p-5 text-sm text-amber-950"
    >
      <p>
        An earlier stock request has not been confirmed. Check its result to
        confirm your stock balances.
      </p>
      <div className="flex gap-2">
        {receipt.attempt && (
          <Button variant="outline" onClick={onReceive}>
            Check receipt
          </Button>
        )}
        {transfer.attempt && (
          <Button variant="outline" onClick={onTransfer}>
            Check transfer
          </Button>
        )}
      </div>
    </div>
  );
}
