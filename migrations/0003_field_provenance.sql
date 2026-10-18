ALTER TABLE observations ADD COLUMN observed_brand TEXT CHECK (observed_brand IS NULL OR length(observed_brand) <= 120);
ALTER TABLE observations ADD COLUMN observed_product_line TEXT CHECK (observed_product_line IS NULL OR length(observed_product_line) <= 120);
ALTER TABLE observations ADD COLUMN observed_variant TEXT CHECK (observed_variant IS NULL OR length(observed_variant) <= 160);
ALTER TABLE observations ADD COLUMN field_provenance_json TEXT NOT NULL DEFAULT '{}' CHECK (length(field_provenance_json) <= 4096);
