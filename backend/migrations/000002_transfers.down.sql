-- Refuse to discard the meaning of existing transfer history.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM stock_movements WHERE kind = 'transfer') THEN
        RAISE EXCEPTION 'Cannot roll back while transfer history exists';
    END IF;
END $$;
ALTER TABLE stock_movements DROP CONSTRAINT movement_kind;
ALTER TABLE stock_movements DROP COLUMN from_location_id;
ALTER TABLE stock_movements DROP COLUMN kind;
ALTER TABLE locations DROP CONSTRAINT locations_fields_nonempty;
DROP INDEX locations_code_normalized_key;
