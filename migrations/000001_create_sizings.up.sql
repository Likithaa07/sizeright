CREATE TABLE sizings (
    id            BIGSERIAL PRIMARY KEY,
    customer_id   TEXT        NOT NULL,
    request       JSONB       NOT NULL,
    result        JSONB       NOT NULL,
    rules_version TEXT        NOT NULL,
    created_by    TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Most common query: latest sizings for a customer.
CREATE INDEX idx_sizings_customer_created ON sizings (customer_id, created_at DESC);
