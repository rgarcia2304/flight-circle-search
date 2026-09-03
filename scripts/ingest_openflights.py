#!/usr/bin/env python3
"""
Ingest OpenFlights airports.dat and routes.dat into CSV files.

Source: https://github.com/jpatokal/openflights
License: Open Database License (ODbL) v1.0 — see https://opendatacommons.org/licenses/odbl/

To comply with the ODbL, we include this attribution notice:
  This product includes data from OpenFlights (https://openflights.org), licensed under ODbL.
"""

import csv
import urllib.request
import sys
import os

AIRPORTS_URL = "https://raw.githubusercontent.com/jpatokal/openflights/master/data/airports.dat"
ROUTES_URL = "https://raw.githubusercontent.com/jpatokal/openflights/master/data/routes.dat"
AIRPORTS_OUT = "data/airports.csv"
ROUTES_OUT = "data/routes.csv"
CHUNK_SIZE = 65536


def download(url: str) -> list[bytes]:
    """Stream-download a URL, returning lines as a list of byte strings."""
    print(f"Downloading {url} ...")
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    with urllib.request.urlopen(req, timeout=60) as resp:
        chunks = []
        while True:
            chunk = resp.read(CHUNK_SIZE)
            if not chunk:
                break
            chunks.append(chunk)
    print(f"  Downloaded {sum(len(c) for c in chunks) / 1024 / 1024:.1f} MB")
    return chunks


def parse_airports(chunks: list[bytes]) -> tuple[set[str], list[dict]]:
    """
    Parse airports.dat. Returns (valid_iata_set, airport_records).
    Drops rows where the IATA field is \\N or empty.
    """
    valid_iata = set()
    records = []

    # Reconstruct as text, split on newlines
    data = b"".join(chunks).decode("utf-8", errors="replace")
    reader = csv.reader(data.splitlines())

    for row in reader:
        if len(row) < 12:
            continue

        iata = row[4].strip()
        if iata == "\\N" or iata == "" or len(iata) != 3:
            continue

        try:
            lat = float(row[6])
            lng = float(row[7])
        except ValueError:
            continue

        if not (-90 <= lat <= 90 and -180 <= lng <= 180):
            continue

        valid_iata.add(iata)
        records.append({
            "iata": iata,
            "name": row[1].strip(),
            "lat": lat,
            "lng": lng,
        })

    return valid_iata, records


def parse_routes(chunks: list[bytes], valid_iata: set[str]) -> list[dict]:
    """
    Parse routes.dat. Returns route records referencing only valid airports.
    Skips rows where origin or destination IATA is \\N or not in valid_iata.
    """
    records = []

    data = b"".join(chunks).decode("utf-8", errors="replace")
    reader = csv.reader(data.splitlines())

    for row in reader:
        if len(row) < 5:
            continue

        origin = row[2].strip()
        dest = row[4].strip()

        if origin == "\\N" or origin == "":
            continue
        if dest == "\\N" or dest == "":
            continue
        if origin not in valid_iata:
            continue
        if dest not in valid_iata:
            continue

        records.append({
            "origin_iata": origin,
            "destination_iata": dest,
        })

    return records


def write_csv(path: str, fieldnames: list[str], records: list[dict]) -> None:
    os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
    with open(path, "w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()
        writer.writerows(records)
    print(f"Wrote {len(records)} rows to {path}")


def main() -> None:
    airport_chunks = download(AIRPORTS_URL)
    route_chunks = download(ROUTES_URL)

    valid_iata, airports = parse_airports(airport_chunks)
    print(f"Airports: {len(valid_iata)} valid IATA codes retained out of {len(airports)} total records")

    routes = parse_routes(route_chunks, valid_iata)
    print(f"Routes: {len(routes)} route pairs referencing valid airports")

    write_csv(AIRPORTS_OUT, ["iata", "name", "lat", "lng"], airports)
    write_csv(ROUTES_OUT, ["origin_iata", "destination_iata"], routes)

    print("\nDone. Spot-check the output files before loading into the database.")


if __name__ == "__main__":
    main()
