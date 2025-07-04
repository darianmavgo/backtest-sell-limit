from backtrader import Strategy, indicators

class SPXLStrategy(Strategy):
    params = (
        ('fast_length', 10),
        ('slow_length', 30),
        ('order_percentage', 0.95),
        ('stop_loss_percent', 0.05),
        ('take_profit_percent', 0.10),
    )

    def __init__(self):
        self.fast_sma = indicators.SMA(self.data.close, period=self.p.fast_length)
        self.slow_sma = indicators.SMA(self.data.close, period=self.p.slow_length)
        self.crossover = indicators.CrossOver(self.fast_sma, self.slow_sma)

        self.order = None
        self.stop_price = None
        self.take_profit_price = None

    def next(self):
        if self.order:
            return

        if not self.position:
            if self.crossover > 0:  # Fast SMA crosses above Slow SMA
                amount_to_invest = self.broker.getcash() * self.p.order_percentage
                size = int(amount_to_invest / self.data.close[0])
                if size > 0:
                    self.order = self.buy(size=size)
                    self.stop_price = self.data.close[0] * (1 - self.p.stop_loss_percent)
                    self.take_profit_price = self.data.close[0] * (1 + self.p.take_profit_percent)
        else:
            if self.crossover < 0:  # Fast SMA crosses below Slow SMA
                self.close()
            elif self.data.close[0] <= self.stop_price:
                self.close()
                self.log(f'STOP LOSS HIT: Closing position at {self.data.close[0]:.2f}')
            elif self.data.close[0] >= self.take_profit_price:
                self.close()
                self.log(f'TAKE PROFIT HIT: Closing position at {self.data.close[0]:.2f}')

    def notify_order(self, order):
        if order.status in [order.Submitted, order.Accepted]:
            return

        if order.status in [order.Completed]:
            if order.isbuy():
                self.log(
                    f'BUY EXECUTED, Price: {order.executed.price:.2f}, Cost: {order.executed.value:.2f}, Comm: {order.executed.comm:.2f}'
                )
            elif order.issell():
                self.log(
                    f'SELL EXECUTED, Price: {order.executed.price:.2f}, Cost: {order.executed.value:.2f}, Comm: {order.executed.comm:.2f}'
                )
            self.bar_executed = len(self)

        elif order.status in [order.Canceled, order.Margin, order.Rejected]:
            self.log('Order Canceled/Margin/Rejected')

        self.order = None

    def notify_trade(self, trade):
        if not trade.isclosed:
            return

        self.log(f'OPERATION PROFIT, GROSS {trade.pnl:.2f}, NET {trade.pnlcomm:.2f}')

    def log(self, txt, dt=None):
        dt = dt or self.datas[0].datetime.date(0)
        print(f'{dt.isoformat()} {txt}')
