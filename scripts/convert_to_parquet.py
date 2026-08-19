"""One-off conversion of the raw source files in data/ into consistently
named Parquet files. Run once after dropping new raw exports into data/.

Usage:
    python3 scripts/convert_to_parquet.py
"""

import pandas as pd
from pathlib import Path

DATA_DIR = Path(__file__).resolve().parent.parent / "data"


def convert_telco_churn():
    df = pd.read_csv(DATA_DIR / "Telco-Customer-Churn.csv")
    df["TotalCharges"] = pd.to_numeric(df["TotalCharges"], errors="coerce")
    df.to_parquet(DATA_DIR / "telco_customer_churn.parquet", index=False)
    print(f"telco_customer_churn.parquet: {len(df)} rows")


def convert_superstore_sales():
    df = pd.read_csv(DATA_DIR / "Superstore-Sales.csv")
    df.to_parquet(DATA_DIR / "superstore_sales.parquet", index=False)
    print(f"superstore_sales.parquet: {len(df)} rows")


def convert_online_retail_ii():
    df = pd.read_csv(DATA_DIR / "online_retail_II.csv")
    df.to_parquet(DATA_DIR / "online_retail_ii.parquet", index=False)
    print(f"online_retail_ii.parquet: {len(df)} rows")


def convert_hr_attrition():
    df = pd.read_csv(DATA_DIR / "WA_Fn-UseC_-HR-Employee-Attrition.csv")
    df.to_parquet(DATA_DIR / "hr_employee_attrition.parquet", index=False)
    print(f"hr_employee_attrition.parquet: {len(df)} rows")


if __name__ == "__main__":
    convert_telco_churn()
    convert_superstore_sales()
    convert_online_retail_ii()
    convert_hr_attrition()
