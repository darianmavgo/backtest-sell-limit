import backtrader as bt
import logging
import sqlite3
from datetime import datetime

class BuySP500Up20(bt.Strategy):
    def __init__(self):
        self.positions_entered = set()  # Set of symbol names
        self.db_path = 'backtest_sell_limits.db'
        self.trade_queue = []
        self.initial_cash = self.broker.getvalue()

    def _write_to_db(self, data: dict):
        """Write a single record to strategy_history table."""
        conn = sqlite3.connect(self.db_path)
        c = conn.cursor()

        c.execute("""
            CREATE TABLE IF NOT EXISTS strategy_history (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                timestamp TEXT,
                strategy_name TEXT,
                symbol TEXT,
                event_type TEXT,
                order_type TEXT,
                status TEXT,
                order_ref INTEGER,
                parent_ref INTEGER,
                price REAL,
                size REAL,
                trade_type TEXT,
                trade_status TEXT,
                quantity REAL,
                value REAL,
                pnl REAL,
                pnl_percent REAL,
                commission REAL,
                trade_date TEXT
            )
        """)

        c.execute("""
            INSERT INTO strategy_history (
                timestamp, strategy_name, symbol, event_type,
                order_type, status, order_ref, parent_ref,
                price, size, trade_type, trade_status,
                quantity, value, pnl, pnl_percent, commission, trade_date
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """, (
            datetime.utcnow().isoformat(),
            data.get('strategy_name'),
            data.get('symbol'),
            data.get('event_type'),
            data.get('order_type'),
            data.get('status'),
            data.get('order_ref'),
            data.get('parent_ref'),
            data.get('price'),
            data.get('size'),
            data.get('trade_type'),
            data.get('trade_status'),
            data.get('quantity'),
            data.get('value'),
            data.get('pnl'),
            data.get('pnl_percent'),
            data.get('commission'),
            data.get('trade_date')
        ))

        conn.commit()
        conn.close()

    def notify_order(self, order):
        """Log order notifications to DB."""
        record = {
            'strategy_name': self.__class__.__name__,
            'symbol': order.data._name,
            'event_type': 'ORDER',
            'order_type': order.ordtypename(),
            'status': order.getstatusname(),
            'order_ref': order.ref,
            'parent_ref': order.parent.ref if order.parent else None,
            'price': None,
            'size': None,
            'trade_type': None,
            'trade_status': None,
            'quantity': None,
            'value': None,
            'pnl': None,
            'pnl_percent': None,
            'commission': None,
            'trade_date': self.datetime.date().strftime('%Y-%m-%d')
        }

        if order.status == order.Completed:
            record['price'] = order.executed.price
            record['size'] = order.executed.size
        self._write_to_db(record)

        # Optional logging
        logging.info(f"Order: {record}")

    def notify_trade(self, trade):
        """Log trade notifications to DB."""
        if trade.isclosed:
            pnl_percent = (trade.pnl / abs(trade.price)) * 100 if trade.price != 0 else 0
            record = {
                'strategy_name': self.__class__.__name__,
                'symbol': trade.data._name,
                'event_type': 'TRADE',
                'order_type': None,
                'status': 'CLOSE',
                'order_ref': None,
                'parent_ref': None,
                'price': trade.price,
                'size': abs(trade.size),
                'trade_type': "LONG" if trade.size > 0 else "SHORT",
                'trade_status': 'CLOSED',
                'quantity': abs(trade.size),
                'value': abs(trade.value),
                'pnl': trade.pnl,
                'pnl_percent': pnl_percent,
                'commission': trade.commission,
                'trade_date': self.datetime.date().strftime('%Y-%m-%d')
            }
        elif trade.isopen:
            record = {
                'strategy_name': self.__class__.__name__,
                'symbol': trade.data._name,
                'event_type': 'TRADE',
                'order_type': None,
                'status': 'OPEN',
                'order_ref': None,
                'parent_ref': None,
                'price': trade.price,
                'size': abs(trade.size),
                'trade_type': "LONG" if trade.size > 0 else "SHORT",
                'trade_status': 'OPEN',
                'quantity': abs(trade.size),
                'value': abs(trade.value),
                'pnl': 0,
                'pnl_percent': 0,
                'commission': trade.commission,
                'trade_date': self.datetime.date().strftime('%Y-%m-%d')
            }

        self._write_to_db(record)

        # Optional logging
        logging.info(f"Trade: {record}")

    def start(self):
        logging.info(f"Starting {self.__class__.__name__}")

    def stop(self):
        final_value = self.broker.getvalue()
        total_return = (final_value - self.initial_cash) / self.initial_cash * 100
        logging.info(f"Final Portfolio Value: {final_value:.2f} | Total Return: {total_return:.2f}%")

    def next(self):
        for d in self.datas:
            symbol = d._name
            if symbol in self.positions_entered:
                continue  # Already entered this symbol — skip!

            # Check if no position exists for this data
            position = self.getposition(d)
            if position.size == 0:
                price = d.close[0]
                take_profit = price * 1.2

                self.buy_bracket(
                    data=d,
                    size=1,
                    price=None,  # Market order
                    stopprice=None,
                    limitprice=take_profit,
                    transmit=True
                )

                logging.info(
                    f"Bracket order for {symbol}: Buy at market, TP at {take_profit:.2f}"
                )

                self.positions_entered.add(symbol)  # Mark this symbol as entered