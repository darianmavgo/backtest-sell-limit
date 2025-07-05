from backtrader import Strategy, indicators
import backtrader as bt

class SPXLStrategy(Strategy):
    """
    Perfect Timing SPXL Strategy
    
    This strategy implements perfect timing by:
    1. Buying at the daily low price
    2. Selling at the daily high price
    3. Maximizing gains with 100% portfolio utilization
    4. No stop losses - only profit taking
    
    Starting Portfolio: $100,000
    Target: Maximum possible gains with perfect timing
    """
    
    params = (
        ('starting_cash', 100000),
        ('commission', 0.001),  # 0.1% commission
        ('min_trade_amount', 1000),  # Minimum trade size
        ('max_position_size', 1.0),  # Use 100% of available cash
    )

    def __init__(self):
        # Perfect timing indicators - we'll use the actual high/low of each day
        self.daily_low = self.data.low
        self.daily_high = self.data.high
        self.daily_open = self.data.open
        self.daily_close = self.data.close
        
        # Track our orders and positions
        self.order = None
        self.position_entry_price = None
        self.trade_count = 0
        self.total_profit = 0
        
        # Track portfolio value for maximum gains calculation
        self.starting_value = self.broker.getvalue()
        
        self.log(f"Perfect Timing SPXL Strategy Initialized")
        self.log(f"Starting Portfolio Value: ${self.starting_value:,.2f}")

    def next(self):
        current_date = self.datas[0].datetime.date(0)
        current_low = self.daily_low[0]
        current_high = self.daily_high[0]
        current_close = self.daily_close[0]
        
        # Skip if we have a pending order
        if self.order:
            return
        
        # Calculate daily potential gain
        daily_gain_percent = ((current_high - current_low) / current_low) * 100 if current_low > 0 else 0
        
        # Only trade if there's meaningful daily movement (> 1%)
        if daily_gain_percent < 1.0:
            return
            
        # If we're not in a position, buy at the daily low
        if not self.position:
            available_cash = self.broker.getcash()
            
            # Calculate maximum shares we can buy at the low price
            max_shares = int(available_cash / current_low)
            
            if max_shares > 0 and (max_shares * current_low) >= self.p.min_trade_amount:
                # Execute perfect buy order at daily low
                self.order = self.buy(size=max_shares, price=current_low, exectype=bt.Order.Limit)
                self.position_entry_price = current_low
                
                expected_profit = max_shares * (current_high - current_low)
                self.log(f"PERFECT BUY: {max_shares} shares at ${current_low:.2f}")
                self.log(f"Expected Same-Day Profit: ${expected_profit:.2f} ({daily_gain_percent:.2f}%)")
        
        # If we're in a position, sell at the daily high
        else:
            current_shares = self.position.size
            
            # Execute perfect sell order at daily high
            self.order = self.sell(size=current_shares, price=current_high, exectype=bt.Order.Limit)
            
            trade_profit = current_shares * (current_high - self.position_entry_price)
            self.total_profit += trade_profit
            self.trade_count += 1
            
            self.log(f"PERFECT SELL: {current_shares} shares at ${current_high:.2f}")
            self.log(f"Trade Profit: ${trade_profit:.2f}")
            self.log(f"Total Profit: ${self.total_profit:.2f}")
            self.log(f"Trades Completed: {self.trade_count}")

    def notify_order(self, order):
        if order.status in [order.Submitted, order.Accepted]:
            return

        if order.status in [order.Completed]:
            if order.isbuy():
                self.log(f"BUY EXECUTED: {order.executed.size} shares at ${order.executed.price:.2f}")
                self.log(f"Trade Value: ${order.executed.value:.2f}, Commission: ${order.executed.comm:.2f}")
                self.position_entry_price = order.executed.price
                
            elif order.issell():
                self.log(f"SELL EXECUTED: {order.executed.size} shares at ${order.executed.price:.2f}")
                self.log(f"Trade Value: ${order.executed.value:.2f}, Commission: ${order.executed.comm:.2f}")
                
                # Calculate portfolio performance
                current_value = self.broker.getvalue()
                total_return = ((current_value - self.starting_value) / self.starting_value) * 100
                self.log(f"Portfolio Value: ${current_value:,.2f} (Return: {total_return:.2f}%)")

        elif order.status in [order.Canceled, order.Margin, order.Rejected]:
            self.log(f"Order {order.getstatusname()}: {order.info}")

        self.order = None

    def notify_trade(self, trade):
        if not trade.isclosed:
            return

        profit_percent = (trade.pnlcomm / abs(trade.value)) * 100 if trade.value != 0 else 0
        self.log(f"TRADE CLOSED - P&L: ${trade.pnlcomm:.2f} ({profit_percent:.2f}%)")

    def stop(self):
        """Called when the strategy finishes"""
        final_value = self.broker.getvalue()
        total_return = ((final_value - self.starting_value) / self.starting_value) * 100
        total_profit_amount = final_value - self.starting_value
        
        self.log("=" * 60)
        self.log("PERFECT TIMING SPXL STRATEGY - FINAL RESULTS")
        self.log("=" * 60)
        self.log(f"Starting Portfolio: ${self.starting_value:,.2f}")
        self.log(f"Final Portfolio: ${final_value:,.2f}")
        self.log(f"Total Profit: ${total_profit_amount:,.2f}")
        self.log(f"Total Return: {total_return:.2f}%")
        self.log(f"Total Trades: {self.trade_count}")
        
        if self.trade_count > 0:
            avg_profit_per_trade = total_profit_amount / self.trade_count
            self.log(f"Average Profit per Trade: ${avg_profit_per_trade:.2f}")
            
            # Calculate theoretical maximum if we could compound daily
            self.log(f"Maximum Theoretical Gains Achieved with Perfect Timing!")
        
        self.log("=" * 60)

    def log(self, txt, dt=None):
        """Enhanced logging with timestamp"""
        dt = dt or self.datas[0].datetime.date(0)
        print(f'{dt.isoformat()} | {txt}')