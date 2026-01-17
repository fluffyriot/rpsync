-- +goose Up
CREATE TABLE table_mappings (
    id UUID PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT now(),

    source_table_name TEXT NOT NULL,
    target_table_name TEXT NOT NULL,
    target_table_code TEXT,

    target_id UUID NOT NULL,
    CONSTRAINT fk_target FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE
);

CREATE TABLE column_mappings (
    id UUID PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT now(),

    table_mapping_id UUID NOT NULL,
    source_column_name TEXT NOT NULL,
    target_column_name TEXT NOT NULL,
    target_column_code TEXT,

    CONSTRAINT fk_table FOREIGN KEY (table_mapping_id)
        REFERENCES table_mappings(id)
        ON DELETE CASCADE
);

-- +goose Down
DROP TABLE table_mappings;
DROP TABLE column_mappings;