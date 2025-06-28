import backtrader as bt
import logging

class PortfolioStrategy(bt.Strategy):
    """A strategy that buys stocks and sets sell limits at 20% above purchase price"""
    
    def __init__(self):
        self.orders = {}  # Keep track of buy orders
        self.sell_orders = {}  # Keep track of sell limit orders
        self.initial_cash = self.broker.getvalue()
        self.stocks_bought = False
        self.purchase_prices = {}  # Keep track of purchase prices
        self.daily_values = []  # Track daily portfolio values
        
    def next(self):
        # Track daily portfolio value
        self.daily_values.append((self.datetime.date(), self.broker.getvalue()))
        
        # Buy stocks on the first day
        if not self.stocks_bought:
            for data in self.datas:
                # Check if we have data
                if len(data) > 0:
                    # Buy 1 share of each stock using bracket order
                    size = 1
                    
                    # Calculate target price (20% above current price)
                    current_price = data.close[0]
                    target_price = current_price * 1.20
                    
                    # Create market buy order first
                    buy_order = self.buy(data=data, size=size)
                    
                    # Create sell limit order at target price
                    sell_order = self.sell(data=data, 
                                         size=size, 
                                         exectype=bt.Order.Limit,
                                         price=target_price,
                                         parent=buy_order,
                                         transmit=True)
                    
                    self.purchase_prices[data._name] = current_price
                    logging.info(
                        f"Placed bracket order for {data._name}", 
                        extra={'symbol': data._name, 'price': current_price}
                    )
                else:
                    logging.warning(f"No data available for {data._name}", extra={'symbol': data._name})
            
            self.stocks_bought = True
    
    def notify_order(self, order):
        """Log order notifications to the database."""
        extra_data = {
            'symbol': order.data._name,
            'order_type': order.ordtypename(),
            'status': order.getstatusname(),
            'order_ref': order.ref,
            'parent_ref': order.parent.ref if order.parent else None
        }
        
        message = f"Order Notification: {order.data._name} - {order.ordtypename()} - {order.getstatusname()}"

        if order.status == order.Completed:
            extra_data['price'] = order.executed.price
            extra_data['size'] = order.executed.size
            message = f"Order Completed: {order.data._name} - {order.ordtypename()} at {order.executed.price}"
            logging.info(message, extra=extra_data)
        elif order.status in [order.Canceled, order.Margin, order.Rejected]:
            logging.warning(message, extra=extra_data)
        else: # Submitted, Accepted, Partial
            logging.info(message, extra=extra_data)

    def stop(self):
        # Calculate return
        final_value = self.broker.getvalue()
        total_return = (final_value - self.initial_cash) / self.initial_cash * 100
        
        # Import here to avoid circular imports
        from portfolio_backtest import save_backtest_results
        
        # Save results
        save_backtest_results(
            "PortfolioStrategy_20_percent_sell_limit",
            self.daily_values,
            self.initial_cash,
            final_value,
            total_return
        ) 