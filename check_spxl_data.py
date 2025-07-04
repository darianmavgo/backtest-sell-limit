import db_manager
import pandas as pd

db = db_manager.SQLiteConnectionManager('backtest_sell_limits.db')

try:
    query = "SELECT * FROM stock_historical_data WHERE symbol = 'SPXL'"
    df = pd.read_sql_query(query, db.get_connection())
    print(df.head())
    print(f"Number of records for SPXL: {len(df)}")
except Exception as e:
    print(f"Error querying database: {e}")
finally:
    db.close()
