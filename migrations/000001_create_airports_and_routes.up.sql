CREATE TABLE airports (
    iata TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    lat DOUBLE PRECISION NOT NULL,
    lng DOUBLE PRECISION NOT NULL
);

CREATE TABLE routes (
    origin_iata TEXT NOT NULL,
    destination_iata TEXT NOT NULL
);

CREATE INDEX idx_routes_origin_dest ON routes (origin_iata, destination_iata);
