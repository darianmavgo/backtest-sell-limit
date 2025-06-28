import logging
import sqlite3
import sys
import os
import time
from datetime import datetime

def load_sql_query(filename):
    """Load SQL query from file in sql/ directory"""
    script_dir = os.path.dirname(os.path.abspath(__file__))
    sql_path = os.path.join(script_dir, "sql", filename)
    with open(sql_path, 'r') as f:
        return f.read().strip()

class DatabaseLogHandler(logging.Handler):
    """A custom logging handler that writes logs to an SQLite database."""
    def __init__(self, db_manager, timeout=5.0):
        super().__init__()
        self.db = db_manager
        self.timeout = timeout
        self.create_table()

    def create_table(self):
        """Create the logs table if it doesn't exist."""
        try:
            self.db.execute(load_sql_query("create_logs_table.sql"))
            self.db.commit()
        except sqlite3.OperationalError as e:
            if "database is locked" in str(e).lower() or "unable to open database file" in str(e).lower():
                print(f"DatabaseLogHandler: Database locked during table creation, attempting cleanup...", file=sys.stderr)
                if self.db.kill_locks():
                    try:
                        self.db.execute(load_sql_query("create_logs_table.sql"))
                        self.db.commit()
                        print(f"DatabaseLogHandler: Table creation successful after cleanup", file=sys.stderr)
                        return  # Success after cleanup
                    except Exception as cleanup_error:
                        print(f"DatabaseLogHandler: Error during cleanup: {cleanup_error}", file=sys.stderr)
            
            print(f"DatabaseLogHandler: Error creating table: {e}", file=sys.stderr)

    def emit(self, record):
        """Emit a log record."""
        max_retries = 3
        retry_delay = 0.1
        
        for attempt in range(max_retries):
            try:
                message = self.format(record)
                self.db.execute(load_sql_query("insert_log_entry.sql"), (
                    datetime.fromtimestamp(record.created).strftime('%Y-%m-%d %H:%M:%S.%f'),
                    record.name,
                    record.levelname,
                    message,
                    getattr(record, 'symbol', None),
                    getattr(record, 'order_type', None),
                    getattr(record, 'status', None),
                    getattr(record, 'price', None),
                    getattr(record, 'size', None),
                    getattr(record, 'order_ref', None),
                    getattr(record, 'parent_ref', None)
                ))
                # Success - break out of retry loop
                break
                    
            except sqlite3.OperationalError as e:
                if "database is locked" in str(e).lower() or "unable to open database file" in str(e).lower():
                    # Database is locked - try simple cleanup first
                    print(f"DatabaseLogHandler: Database locked, attempting cleanup...", file=sys.stderr)
                    if self.db.kill_locks():
                        # Wait a moment for cleanup
                        time.sleep(0.5)
                        print(f"DatabaseLogHandler: Simple cleanup completed", file=sys.stderr)
                        continue
                
                if attempt < max_retries - 1:  # Not the last attempt
                    time.sleep(retry_delay)
                    retry_delay *= 2  # Exponential backoff
                    continue
                else:
                    # Last attempt failed - print error but don't crash
                    print(f"DatabaseLogHandler: Failed to write log after {max_retries} attempts: {e}", file=sys.stderr)
                    print(f"File exists: {os.path.exists(self.db.db_file)}", file=sys.stderr)
                    
            except Exception as e:
                # Other exceptions - print and continue
                print(f"DatabaseLogHandler: Unexpected error: {e}", file=sys.stderr)
                import traceback
                traceback.print_exc(file=sys.stderr)
                break

    def __del__(self):
        """Nothing to clean up since we're using the global connection."""
        pass 