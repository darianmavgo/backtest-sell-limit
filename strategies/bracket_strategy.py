import backtrader as bt
import logging

class BracketStrategy(bt.Strategy):
    """
    This strategy implements a bracket order that buys 1 share of every
    available stock at the market price. Once a buy order is executed,
    it automatically places a limit sell order to take profit at 20%
    above the execution price. It does not include a stop-loss leg.
    """

    def log(self, txt, dt=None):
        """ Logging function for this strategy """
        dt = dt or self.datas[0].datetime.date(0)
        print(f'{dt.isoformat()} - {txt}')

    def __init__(self):
        """
        Conception: __init__ is called when the strategy is created.
        We can initialize attributes here. For this strategy, no
        special indicators or attributes are needed initially.
        """
        self.log('Strategy Initialized')
        # This dictionary will keep track of the take-profit sell order
        # associated with each stock's buy order.
        self.profit_takers = {}

    def notify_order(self, order):
        """
        This method is called for any status change in an order.
        It is the heart of our bracket order logic.
        """
        # 1. If order is submitted/accepted, do nothing
        if order.status in [order.Submitted, order.Accepted]:
            return

        # 2. Check if an order has been completed
        if order.status in [order.Completed]:
            if order.isbuy():
                # This was a buy order that has been executed
                self.log(
                    f'BUY EXECUTED for {order.data._name}: '
                    f'Price: {order.executed.price:.2f}, '
                    f'Size: {order.executed.size}, '
                    f'Cost: {order.executed.value:.2f}'
                )

                # --- BRACKET LOGIC ---
                # Calculate the take profit price (20% above execution price)
                profit_price = order.executed.price * 1.20

                # Create the corresponding limit sell order to take profit
                self.log(
                    f'CREATING TAKE PROFIT SELL for {order.data._name} '
                    f'at Price: {profit_price:.2f}'
                )
                profit_order = self.sell(
                    data=order.data,
                    size=order.executed.size,
                    price=profit_price,
                    exectype=bt.Order.Limit
                )
                # Store the reference to the profit-taking order
                self.profit_takers[order.data._name] = profit_order

            elif order.issell():
                # This was a sell order that has been executed
                self.log(
                    f'SELL EXECUTED (Take Profit) for {order.data._name}: '
                    f'Price: {order.executed.price:.2f}, '
                    f'Size: {order.executed.size}, '
                    f'Value: {order.executed.value:.2f}'
                )
                # Since the sell order (our profit taker) executed,
                # we can remove its reference.
                if order.data._name in self.profit_takers:
                    del self.profit_takers[order.data._name]

        # 3. Handle other order statuses like Canceled, Margin, or Rejected
        elif order.status in [order.Canceled, order.Margin, order.Rejected]:
            self.log(f'Order for {order.data._name} was Canceled/Margin/Rejected')

    def start(self):
        """
        Birth: This method is called once the minimum period of all
        indicators is met. Since we have no indicators, it's called
        at the very beginning. We use it to place our initial buy orders.
        """
        self.log('Strategy Starting. Placing initial buy orders.')
        for d in self.datas:
            # Place a market order to buy 1 share of each stock
            self.buy(data=d, size=1)

    def stop(self):
        """
        Death: This method is called at the end of the backtest.
        A good place to print final results.
        """
        self.log(f'Strategy Stopping. Final Portfolio Value: {self.broker.getvalue():.2f}') 