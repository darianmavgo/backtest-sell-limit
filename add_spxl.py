
import db_manager

db = db_manager.SQLiteConnectionManager('backtest_sell_limits.db')
db.execute("INSERT OR IGNORE INTO ticker_list (symbol, company_name) VALUES (?, ?)", ('SPXL', 'Direxion Daily S&P 500 Bull 3X Shares'))
db.commit()
