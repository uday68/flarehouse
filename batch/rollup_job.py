#!/usr/bin/env python3
"""
FlareHouse Cold Partition Rollup Job (PySpark)
-----------------------------------------------
Reads exported Parquet files of cold ClickHouse partitions (e.g., archived to
object storage such as GCS/S3) and aggregates granular telemetry into hourly
rollups for historical reporting and BI federation.

Scope Note:
This job is intended for localized scheduled batch processing and offline partition
consolidation. It is tested in local standalone mode; it does not claim multi-node
cluster-scale benchmarks.
"""

import argparse
import sys
from pyspark.sql import SparkSession
from pyspark.sql import functions as F
from pyspark.sql.types import (
    StructType,
    StructField,
    StringType,
    IntegerType,
    LongType,
    FloatType,
    TimestampType,
)

# Standard schema matching ClickHouse flarehouse.edge_http_logs Parquet exports
TELEMETRY_SCHEMA = StructType([
    StructField("timestamp", TimestampType(), True),
    StructField("tenant_id", StringType(), True),
    StructField("request_id", StringType(), True),
    StructField("client_ip", StringType(), True),
    StructField("host", StringType(), True),
    StructField("path", StringType(), True),
    StructField("method", StringType(), True),
    StructField("status_code", IntegerType(), True),
    StructField("response_bytes", LongType(), True),
    StructField("duration_ms", FloatType(), True),
    StructField("edge_colo", StringType(), True),
    StructField("country", StringType(), True),
    StructField("cache_status", StringType(), True),
    StructField("threat_score", IntegerType(), True),
])


def get_spark_session(app_name: str = "FlareHouse-Hourly-Rollup") -> SparkSession:
    """Creates a local standalone SparkSession with optimized partition settings."""
    return (
        SparkSession.builder.appName(app_name)
        .master("local[*]")
        .config("spark.sql.shuffle.partitions", "4")
        .config("spark.driver.memory", "2g")
        .getOrCreate()
    )


def compute_hourly_rollups(df):
    """
    Groups raw telemetry events by hourly window and dimensions, computing
    aggregations for request volume, total bytes, and latency metrics.
    """
    # Normalize timestamp column and truncate to the nearest hour
    with_hour = df.withColumn(
        "rollup_hour", F.date_trunc("hour", F.col("timestamp"))
    )

    rollups = with_hour.groupBy(
        "rollup_hour",
        "tenant_id",
        "edge_colo",
        "status_code",
        "country",
    ).agg(
        F.count(F.lit(1)).alias("total_requests"),
        F.sum("response_bytes").alias("total_egress_bytes"),
        F.round(F.avg("duration_ms"), 2).alias("avg_duration_ms"),
        F.round(F.min("duration_ms"), 2).alias("min_duration_ms"),
        F.round(F.max("duration_ms"), 2).alias("max_duration_ms"),
    ).orderBy("rollup_hour", "tenant_id", "edge_colo")

    return rollups


def main():
    parser = argparse.ArgumentParser(
        description="Aggregate FlareHouse cold partition Parquet telemetry into hourly rollups."
    )
    parser.add_argument(
        "--input",
        type=str,
        default="./data/raw_partitions",
        help="Path to input Parquet files or directory of partitions",
    )
    parser.add_argument(
        "--output",
        type=str,
        default="./data/hourly_rollups",
        help="Path to write consolidated Parquet hourly rollups",
    )
    parser.add_argument(
        "--format",
        type=str,
        default="parquet",
        choices=["parquet", "csv"],
        help="Output format (parquet or csv)",
    )
    args = parser.parse_args()

    print(f"[*] Starting FlareHouse PySpark Rollup Job")
    print(f"[*] Input Path:  {args.input}")
    print(f"[*] Output Path: {args.output}")

    spark = get_spark_session()
    spark.sparkContext.setLogLevel("WARN")

    try:
        raw_df = spark.read.schema(TELEMETRY_SCHEMA).parquet(args.input)
        record_count = raw_df.count()
        print(f"[+] Successfully loaded {record_count:,} raw telemetry records.")

        if record_count == 0:
            print("[!] Warning: Input dataset is empty. Exiting without writing.")
            sys.exit(0)

        rollup_df = compute_hourly_rollups(raw_df)

        print("[*] Sample Aggregated Results:")
        rollup_df.show(5, truncate=False)

        # Write output partitioned by tenant_id
        writer = rollup_df.write.mode("overwrite")
        if args.format == "parquet":
            writer.partitionBy("tenant_id").parquet(args.output)
        else:
            writer.partitionBy("tenant_id").option("header", "true").csv(args.output)

        print(f"[+] Hourly rollups successfully written to: {args.output}")

    except Exception as e:
        print(f"[-] Rollup job execution error: {e}", file=sys.stderr)
        sys.exit(1)
    finally:
        spark.stop()


if __name__ == "__main__":
    main()
