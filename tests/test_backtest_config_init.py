import unittest
import os
import json
import sys
import os
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..', '..')))
from backtest_framework import BacktestConfig

class TestBacktestConfigInitialization(unittest.TestCase):
    def test_initialization_with_real_config(self):
        # Assuming config.json is in the same directory as backtest_framework.py
        # or the current working directory when the test is run.
        # For this test, we'll assume it's in the current working directory.
        config_path = "config.json"
        
        # Read the actual config.json to get expected values
        with open(config_path, 'r') as f:
            expected_config = json.load(f)

        config = BacktestConfig(config_path)

        self.assertEqual(config.database_path, "backtest_strategies.db")
        self.assertEqual(config.start_date, "2020-01-01")
        self.assertEqual(config.end_date, "2025-07-05")
        self.assertEqual(config.initial_cash, 100000)
        self.assertEqual(config.commission, 0.001)
        self.assertEqual(config.date_format, "YYYY-MM-DD")

if __name__ == '__main__':
    unittest.main()