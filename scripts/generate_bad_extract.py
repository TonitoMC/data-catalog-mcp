"""Generates data/meridian_mobile_subscribers_raw.parquet: a copy of the
subscriber base with five deliberate, reproducible inconsistencies, each
targeting a distinct validation rule type. Used to demo validate_dataset.

Requires meridian_mobile_subscribers.parquet to already exist (run
convert_to_parquet.py first).

Usage:
    python3 scripts/generate_bad_extract.py
"""

import numpy as np
import pandas as pd
from pathlib import Path

DATA_DIR = Path(__file__).resolve().parent.parent / "data"
SEED = 42


def main():
    rng = np.random.default_rng(SEED)
    df = pd.read_csv(DATA_DIR / "Telco-Customer-Churn.csv")
    df["TotalCharges"] = pd.to_numeric(df["TotalCharges"], errors="coerce")

    n = len(df)

    # 1. not_null violation: blank out some MonthlyCharges values.
    null_idx = rng.choice(n, size=15, replace=False)
    df.loc[null_idx, "MonthlyCharges"] = np.nan

    # 2. range violation: implausible negative/huge tenure values.
    range_idx = rng.choice(n, size=10, replace=False)
    df.loc[range_idx, "tenure"] = rng.choice([-5, 9999], size=10)

    # 3. allowed_values violation: invalid Contract category.
    allowed_idx = rng.choice(n, size=8, replace=False)
    df.loc[allowed_idx, "Contract"] = "Biannual"

    # 4. regex violation: malformed customerID (should match \d{4}-[A-Z]{5}).
    regex_idx = rng.choice(n, size=12, replace=False)
    df.loc[regex_idx, "customerID"] = "bad_id_" + df.loc[regex_idx].index.astype(str)

    # 5. type_check violation: non-numeric junk in TotalCharges.
    type_idx = rng.choice(n, size=6, replace=False)
    df["TotalCharges"] = df["TotalCharges"].astype(object)
    df.loc[type_idx, "TotalCharges"] = "N/A"

    # TotalCharges now mixes floats and the literal "N/A" string, which
    # Arrow can't type-infer on its own — force it to a plain string column
    # so the type_check rule has something realistic to catch.
    df["TotalCharges"] = df["TotalCharges"].astype(str).replace("nan", None)

    out_path = DATA_DIR / "meridian_mobile_subscribers_raw.parquet"
    df.to_parquet(out_path, index=False)
    print(f"{out_path.name}: {n} rows, 5 inconsistency types injected (seed={SEED})")


if __name__ == "__main__":
    main()
