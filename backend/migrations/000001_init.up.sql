CREATE TABLE items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sku varchar(64) NOT NULL UNIQUE CHECK (sku = upper(btrim(sku)) AND length(sku) > 0),
    name varchar(200) NOT NULL CHECK (length(btrim(name)) > 0),
    category varchar(100) NOT NULL CHECK (length(btrim(category)) > 0),
    unit varchar(32) NOT NULL CHECK (length(btrim(unit)) > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX items_active_order ON items (created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX items_active_category ON items (category) WHERE deleted_at IS NULL;
CREATE TABLE locations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code varchar(32) NOT NULL UNIQUE,
    zone varchar(100) NOT NULL,
    type varchar(50) NOT NULL
);
CREATE TABLE stocks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id uuid NOT NULL REFERENCES items(id),
    location_id uuid NOT NULL REFERENCES locations(id),
    qty numeric(18,3) NOT NULL CHECK (qty >= 0 AND qty <> 'NaN'::numeric),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (item_id, location_id)
);
CREATE TABLE stock_movements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id uuid NOT NULL REFERENCES items(id),
    location_id uuid NOT NULL REFERENCES locations(id),
    qty numeric(18,3) NOT NULL CHECK (qty > 0 AND qty <> 'NaN'::numeric),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX stock_movements_item ON stock_movements(item_id, created_at);
CREATE TABLE idempotency_records (
    key varchar(128) PRIMARY KEY,
    payload_hash text NOT NULL,
    response jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
