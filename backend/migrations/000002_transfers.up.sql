CREATE UNIQUE INDEX locations_code_normalized_key ON locations(upper(btrim(code)));
ALTER TABLE locations ADD CONSTRAINT locations_fields_nonempty CHECK (btrim(code) <> '' AND btrim(zone) <> '' AND btrim(type) <> '');
ALTER TABLE stock_movements ADD COLUMN kind text NOT NULL DEFAULT 'receive';
ALTER TABLE stock_movements ADD COLUMN from_location_id uuid REFERENCES locations(id);
ALTER TABLE stock_movements ADD CONSTRAINT movement_kind CHECK (
    (kind = 'receive' AND from_location_id IS NULL) OR
    (kind = 'transfer' AND from_location_id IS NOT NULL AND from_location_id <> location_id)
);
