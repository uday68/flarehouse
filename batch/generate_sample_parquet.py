#!/usr/bin/env python3
"""
Generate synthetic Parquet data matching FlareHouse ClickHouse export schema.
Used for local testing of batch/rollup_job.py.
"""

import os
import random
from datetime import datetime, timedelta
import pandas as pd

def generate_sample_parquet(output_path="./data/raw_partitions/sample_partition.parquet", num_records=2500):
    os.makedirs(os.path.dirname(output_path), exist_ok=True)

    base_time = datetime.utcnow().replace(minute=0, second=0, microsecond=0)
    tenants = ["internal-platform", "enterprise-corp", "fast-startup-io", "global-bank"]
    colos = ["SJC", "IAD", "LHR", "FRA", "SIN", "BLR", "NRT", "SYD"]
    countries = ["US", "GB", "DE", "SG", "IN", "JP", "AU"]
    methods = ["GET", "POST", "GET", "HEAD"]
    statuses = [200, 200, 200, 301, 403, 404, 500]

    rows = []
    for i in range(num_records):
        # spread over the last 3 hours
        event_time = base_time - timedelta(minutes=random.randint(0, 180), seconds=random.randint(0, 59))
        rows.append({
            "timestamp": event_time,
            "tenant_id": random.choice(tenants),
            "request_id": f"req-{i:06d}",
            "client_ip": f"{random.randint(1, 220)}.{random.randint(1, 255)}.{random.randint(1, 255)}.{random.randint(1, 255)}",
            "host": f"api.service-{random.randint(1, 5)}.internal.net",
            "path": f"/v1/telemetry/{random.randint(1, 50)}",
            "method": random.choice(methods),
            "status_code": random.choice(statuses),
            "response_bytes": random.randint(300, 75000),
            "duration_ms": round(random.uniform(2.5, 120.0), 2),
            "edge_colo": random.choice(colos),
            "country": random.choice(countries),
            "cache_status": random.choice(["HIT", "MISS", "BYPASS"]),
            "threat_score": random.randint(0, 10),
        })

    df = pd.DataFrame(rows)
    df["timestamp"] = pd.to_datetime(df["timestamp"])
    df.to_parquet(output_path, engine="pyarrow", index=False)
    print(f"[+] Successfully generated {num_records} sample records in: {output_path}")

if __name__ == "__main__":
    generate_sample_parquet()
