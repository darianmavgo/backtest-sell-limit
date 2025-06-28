import sqlite3
import os
import sys
import time
import atexit

class SQLiteConnectionManager:
    """Manages a single global SQLite connection with automatic reconnection."""
    def __init__(self, db_file, timeout=5.0):
        self.db_file = db_file
        self.timeout = timeout
        self._conn = None
        # Get the project root directory (where the db file is)
        self.project_root = os.path.dirname(os.path.abspath(self.db_file)) if os.path.isabs(self.db_file) else os.getcwd()
        # Ensure the directory exists
        db_dir = os.path.dirname(self.db_file)
        if db_dir and not os.path.exists(db_dir):
            os.makedirs(db_dir, exist_ok=True)
        self.connect()
        atexit.register(self.close)
    
    def connect(self):
        """Establish a new connection if one doesn't exist."""
        try:
            if self._conn is None:
                self._conn = sqlite3.connect(
                    self.db_file,
                    timeout=self.timeout,
                    check_same_thread=False,  # Allow multi-threading
                    isolation_level='DEFERRED'  # Use explicit transaction control
                )
        except Exception as e:
            print(f"Error connecting to database: {e}", file=sys.stderr)
            raise
    
    def get_connection(self):
        """Get the current connection, reconnecting if necessary."""
        if self._conn is None:
            self.connect()
            if self._conn is None:
                raise sqlite3.OperationalError("Failed to establish database connection")
        try:
            # Test the connection
            self._conn.execute("SELECT 1")
        except (sqlite3.OperationalError, sqlite3.ProgrammingError, AttributeError):
            # Reconnect if the connection is dead or was closed
            self.connect()
            if self._conn is None:
                raise sqlite3.OperationalError("Failed to re-establish database connection")
        return self._conn
    
    def close(self):
        """Close the connection if it exists."""
        if self._conn is not None:
            try:
                self._conn.close()
            except Exception:
                pass
            finally:
                self._conn = None

    def execute(self, sql, parameters=()):
        """Execute a SQL statement with proper connection handling."""
        conn = self.get_connection()
        return conn.execute(sql, parameters)

    def commit(self):
        """Commit the current transaction."""
        conn = self.get_connection()
        conn.commit()

    def rollback(self):
        """Rollback the current transaction."""
        conn = self.get_connection()
        conn.rollback()

    def load_sql_query(self, filename):
        """Load SQL query from file in sql/ directory"""
        sql_path = os.path.join(self.project_root, "sql", filename)
        try:
            with open(sql_path, 'r') as f:
                return f.read().strip()
        except FileNotFoundError:
            print(f"SQL file not found: {sql_path}", file=sys.stderr)
            raise

    def execute_sql_file(self, filename, parameters=()):
        """Execute a SQL query from a file with proper connection handling."""
        sql = self.load_sql_query(filename)
        return self.execute(sql, parameters)

    def kill_locks(self):
        """Clear database locks using simple file operations"""
        try:
            print("🔒 Database locked - attempting to clear locks...", file=sys.stderr)
            
            # Get database file path
            db_base = os.path.splitext(self.db_file)[0]
            
            # List of potential lock files
            lock_files = [
                f"{self.db_file}-journal",
                f"{db_base}.db-wal",
                f"{db_base}.db-shm",
                f"{db_base}.db-journal"
            ]
            
            removed_files = []
            for lock_file in lock_files:
                if os.path.exists(lock_file):
                    try:
                        os.remove(lock_file)
                        removed_files.append(lock_file)
                        print(f"🗑️ Removed: {os.path.basename(lock_file)}", file=sys.stderr)
                    except Exception as e:
                        print(f"⚠️ Could not remove {os.path.basename(lock_file)}: {e}", file=sys.stderr)
            
            if removed_files:
                print(f"✅ Removed {len(removed_files)} lock files", file=sys.stderr)
                time.sleep(1)  # Give time for cleanup
                return True
            else:
                print("ℹ️ No lock files found to remove", file=sys.stderr)
                return True  # Not an error if no lock files exist
                
        except Exception as e:
            print(f"❌ Error during lock cleanup: {e}", file=sys.stderr)
            return False 