import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  ArrowDownToLine,
  ArrowRightLeft,
  ArrowLeft,
  MapPin,
  Package,
  Pencil,
  RefreshCw,
  Trash2,
} from "lucide-react";
import { useItem, useStock } from "@/hooks/use-warehouse";
import { IconButton } from "@/components/icon-button";
import { Button } from "@/components/ui/button";
import { ErrorState, Loading } from "@/components/states";
import { DeleteDialog } from "@/components/delete-dialog";
import { TransferDialog } from "@/components/transfer-dialog";
import { PendingStockNotice } from "@/components/pending-stock-notice";
import { ReceiveDialog } from "@/components/receive-dialog";
import { quantity } from "@/lib/utils";
export default function ItemDetailPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const itemQuery = useItem(id);
  const stock = useStock(id);
  const [transferring, setTransferring] = useState(false);
  const [receiving, setReceiving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const item = itemQuery.data?.data;
  return (
    <div className="page">
      <IconButton label="Back to inventory" asChild className="-ml-3 mb-5">
        <Link to="/items">
          <ArrowLeft />
        </Link>
      </IconButton>
      {itemQuery.isPending ? (
        <div className="panel">
          <Loading />
        </div>
      ) : itemQuery.isError ? (
        <ErrorState
          error={itemQuery.error}
          retry={() => void itemQuery.refetch()}
        />
      ) : (
        item && (
          <>
            <div className="mb-9 flex items-start justify-between gap-6">
              <div>
                <p className="eyebrow mb-3">Item overview</p>
                <h1 className="text-[32px] font-semibold tracking-tight">
                  {item.name}
                </h1>
                <p className="mt-3 font-mono text-sm text-muted-foreground">
                  {item.sku}
                </p>
              </div>
              <div className="flex gap-2">
                <IconButton label="Edit item" variant="outline" asChild>
                  <Link to={`/items/${id}/edit`}>
                    <Pencil />
                  </Link>
                </IconButton>
                <IconButton
                  label="Delete item"
                  variant="outline"
                  className="hover:border-red-200 hover:bg-red-50 hover:text-red-600"
                  onClick={() => setDeleting(true)}
                >
                  <Trash2 />
                </IconButton>
              </div>
            </div>
            <PendingStockNotice
              itemID={id}
              onReceive={() => setReceiving(true)}
              onTransfer={() => setTransferring(true)}
            />
            <div className="grid items-start gap-6 lg:grid-cols-[1fr_300px]">
              <section className="panel" aria-labelledby="stock-heading">
                <div className="flex flex-wrap items-center justify-between gap-4 border-b px-6 py-5">
                  <div>
                    <h2 id="stock-heading" className="font-semibold">
                      Stock by location
                    </h2>
                    <p className="mt-1 text-xs text-muted-foreground">
                      Every quantity, in its place.
                    </p>
                  </div>
                  <div className="flex items-center gap-2">
                    <IconButton
                      label="Refresh stock"
                      disabled={stock.isFetching}
                      onClick={() => void stock.refetch()}
                    >
                      <RefreshCw
                        className={stock.isFetching ? "animate-spin" : ""}
                      />
                    </IconButton>
                    <IconButton
                      label="Transfer stock"
                      variant="outline"
                      onClick={() => setTransferring(true)}
                    >
                      <ArrowRightLeft />
                    </IconButton>
                    <Button onClick={() => setReceiving(true)}>
                      <ArrowDownToLine />
                      Receive stock
                    </Button>
                  </div>
                </div>
                {stock.isPending ? (
                  <Loading rows={3} />
                ) : stock.isError ? (
                  <ErrorState
                    error={stock.error}
                    retry={() => void stock.refetch()}
                  />
                ) : (
                  <>
                    <div className="border-b bg-[#fbfcfe] px-7 py-7">
                      <p className="eyebrow">Total on hand</p>
                      <p
                        className="mt-3 text-4xl font-semibold tracking-tight"
                        data-testid="stock-total"
                      >
                        {quantity(stock.data.data.total)}
                        <span className="ml-2 text-base font-normal tracking-normal text-muted-foreground">
                          {stock.data.data.unit}
                        </span>
                      </p>
                    </div>
                    {stock.data.data.locations.length === 0 ? (
                      <div className="px-8 py-14 text-center">
                        <MapPin
                          className="mx-auto mb-4 size-7 text-muted-foreground"
                          aria-hidden="true"
                        />
                        <h3 className="font-medium">No stock received yet</h3>
                        <p className="mt-2 text-sm text-muted-foreground">
                          Receive your first delivery to assign stock to a
                          location.
                        </p>
                      </div>
                    ) : (
                      <table className="w-full text-left text-sm">
                        <thead className="text-xs text-muted-foreground">
                          <tr>
                            <th className="px-6 py-4 font-medium">Location</th>
                            <th className="px-4 py-4 font-medium">Zone</th>
                            <th className="px-6 py-4 text-right font-medium">
                              Quantity
                            </th>
                          </tr>
                        </thead>
                        <tbody>
                          {stock.data.data.locations.map((balance) => (
                            <tr key={balance.id} className="border-t">
                              <td className="px-6 py-5">
                                <span className="flex items-center gap-2 font-medium">
                                  <MapPin
                                    className="size-4 text-muted-foreground"
                                    aria-hidden="true"
                                  />
                                  {balance.location.code}
                                </span>
                                <span className="mt-1 block pl-6 text-xs text-muted-foreground">
                                  {balance.location.type}
                                </span>
                              </td>
                              <td className="px-4 py-5 text-muted-foreground">
                                {balance.location.zone}
                              </td>
                              <td className="px-6 py-5 text-right font-medium tabular-nums">
                                {quantity(balance.qty)}{" "}
                                <span className="font-normal text-muted-foreground">
                                  {item.unit}
                                </span>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    )}
                  </>
                )}
              </section>
              <aside className="panel p-6">
                <div className="mb-5 flex items-center gap-2">
                  <Package
                    className="size-[18px] text-muted-foreground"
                    aria-hidden="true"
                  />
                  <h2 className="text-sm font-semibold">Item details</h2>
                </div>
                <dl className="space-y-5 text-sm">
                  {[
                    ["Category", item.category],
                    ["Unit of measure", item.unit],
                    [
                      "Created",
                      new Intl.DateTimeFormat("en", {
                        dateStyle: "medium",
                      }).format(new Date(item.created_at)),
                    ],
                  ].map(([label, value]) => (
                    <div key={label}>
                      <dt className="mb-1.5 text-xs text-muted-foreground">
                        {label}
                      </dt>
                      <dd className="font-medium">{value}</dd>
                    </div>
                  ))}
                </dl>
              </aside>
            </div>
            <TransferDialog
              key={`transfer:${id}`}
              open={transferring}
              onOpenChange={setTransferring}
              itemID={id}
              unit={item.unit}
              stock={stock.data?.data}
            />
            <ReceiveDialog
              key={`receive:${id}`}
              open={receiving}
              onOpenChange={setReceiving}
              itemID={id}
              unit={item.unit}
            />
            <DeleteDialog
              item={deleting ? item : null}
              onClose={() => setDeleting(false)}
              onDeleted={() => navigate("/items")}
            />
          </>
        )
      )}
    </div>
  );
}
