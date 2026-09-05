"""One-off conversion of the raw source files in data/ into consistently
named Parquet files. Run once after dropping new raw exports into data/.

Usage:
    python3 scripts/convert_to_parquet.py
"""

import pandas as pd
from pathlib import Path

DATA_DIR = Path(__file__).resolve().parent.parent / "data"


def convert_meridian_mobile_subscribers():
    df = pd.read_csv(DATA_DIR / "Telco-Customer-Churn.csv")
    df["TotalCharges"] = pd.to_numeric(df["TotalCharges"], errors="coerce")
    df.to_parquet(DATA_DIR / "meridian_mobile_subscribers.parquet", index=False)
    print(f"meridian_mobile_subscribers.parquet: {len(df)} rows")


def convert_meridian_retail_sales():
    df = pd.read_csv(DATA_DIR / "Superstore-Sales.csv")
    df.to_parquet(DATA_DIR / "meridian_retail_sales.parquet", index=False)
    print(f"meridian_retail_sales.parquet: {len(df)} rows")


def convert_meridian_commerce_transactions():
    df = pd.read_csv(DATA_DIR / "online_retail_II.csv")
    df.to_parquet(DATA_DIR / "meridian_commerce_transactions.parquet", index=False)
    print(f"meridian_commerce_transactions.parquet: {len(df)} rows")


def convert_corporate_hr_attrition():
    df = pd.read_csv(DATA_DIR / "WA_Fn-UseC_-HR-Employee-Attrition.csv")
    df.to_parquet(DATA_DIR / "corporate_hr_attrition.parquet", index=False)
    print(f"corporate_hr_attrition.parquet: {len(df)} rows")


if __name__ == "__main__":
    convert_meridian_mobile_subscribers()
    convert_meridian_retail_sales()
    convert_meridian_commerce_transactions()
    convert_corporate_hr_attrition()
