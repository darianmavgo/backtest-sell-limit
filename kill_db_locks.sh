#!/bin/bash

# kill_db_locks.sh - Script to kill all locks on the SQLite database
# Usage: ./kill_db_locks.sh [database_name]

set -e  # Exit on any error

# Default database name
DB_NAME="backtest_sell_limits.db"

# Use provided database name if given
if [ $# -eq 1 ]; then
    DB_NAME="$1"
fi

echo "🔒 Killing all locks on database: $DB_NAME"
echo "================================================"

# Check if database exists
if [ ! -f "$DB_NAME" ]; then
    echo "❌ Database file '$DB_NAME' not found!"
    exit 1
fi

echo "📁 Database file found: $DB_NAME"

# 1. Kill any processes using the database files
echo ""
echo "🔍 Step 1: Checking for processes using database files..."
DB_PROCESSES=$(lsof "$DB_NAME"* 2>/dev/null | grep -v COMMAND || true)

if [ -n "$DB_PROCESSES" ]; then
    echo "⚠️  Found processes using database files:"
    echo "$DB_PROCESSES"
    
    # Extract PIDs and kill them
    PIDS=$(echo "$DB_PROCESSES" | awk '{print $2}' | sort -u)
    for PID in $PIDS; do
        echo "💀 Killing process $PID..."
        kill -TERM "$PID" 2>/dev/null || true
        sleep 1
        # Force kill if still running
        if kill -0 "$PID" 2>/dev/null; then
            echo "💀 Force killing process $PID..."
            kill -KILL "$PID" 2>/dev/null || true
        fi
    done
else
    echo "✅ No processes found using database files"
fi

# 2. Remove SQLite lock files
echo ""
echo "🧹 Step 2: Removing SQLite lock files..."

# Remove WAL file
if [ -f "${DB_NAME}-wal" ]; then
    echo "🗑️  Removing WAL file: ${DB_NAME}-wal"
    rm -f "${DB_NAME}-wal"
else
    echo "✅ No WAL file found"
fi

# Remove SHM file
if [ -f "${DB_NAME}-shm" ]; then
    echo "🗑️  Removing SHM file: ${DB_NAME}-shm"
    rm -f "${DB_NAME}-shm"
else
    echo "✅ No SHM file found"
fi

# Remove journal file (if exists)
if [ -f "${DB_NAME}-journal" ]; then
    echo "🗑️  Removing journal file: ${DB_NAME}-journal"
    rm -f "${DB_NAME}-journal"
else
    echo "✅ No journal file found"
fi

# 3. Force SQLite to release any internal locks
echo ""
echo "🔧 Step 3: Forcing SQLite to release internal locks..."

# Try to connect and run PRAGMA commands to clear locks
sqlite3 "$DB_NAME" << 'EOF' 2>/dev/null || echo "⚠️  Could not run SQLite commands (database may be severely locked)"
-- Force checkpoint to clear WAL
PRAGMA wal_checkpoint(TRUNCATE);

-- Optimize database (can help clear locks)
PRAGMA optimize;

-- Check if database is accessible
SELECT 'Database is accessible' as status;

-- Exit
.quit
EOF

# 4. Kill any remaining SQLite processes
echo ""
echo "🔍 Step 4: Checking for any remaining SQLite processes..."

SQLITE_PROCESSES=$(ps aux | grep -i sqlite | grep -v grep | grep -v "$0" || true)
if [ -n "$SQLITE_PROCESSES" ]; then
    echo "⚠️  Found SQLite processes:"
    echo "$SQLITE_PROCESSES"
    
    # Kill SQLite processes related to our database
    ps aux | grep -i sqlite | grep -v grep | grep -v "$0" | while read line; do
        PID=$(echo "$line" | awk '{print $2}')
        COMMAND=$(echo "$line" | awk '{for(i=11;i<=NF;i++) printf "%s ", $i; print ""}')
        
        # Check if the process is related to our database
        if echo "$COMMAND" | grep -q "$DB_NAME"; then
            echo "💀 Killing SQLite process $PID: $COMMAND"
            kill -TERM "$PID" 2>/dev/null || true
            sleep 1
            if kill -0 "$PID" 2>/dev/null; then
                kill -KILL "$PID" 2>/dev/null || true
            fi
        fi
    done
else
    echo "✅ No SQLite processes found"
fi

# 5. Final verification
echo ""
echo "🔍 Step 5: Final verification..."

# Check if we can connect to the database
if sqlite3 "$DB_NAME" "SELECT 1;" >/dev/null 2>&1; then
    echo "✅ Database is now accessible!"
else
    echo "❌ Database may still have issues"
fi

# Check for remaining lock files
REMAINING_LOCKS=$(ls -la "${DB_NAME}"* 2>/dev/null | grep -E '\-(wal|shm|journal)$' || true)
if [ -n "$REMAINING_LOCKS" ]; then
    echo "⚠️  Some lock files may still exist:"
    echo "$REMAINING_LOCKS"
else
    echo "✅ No lock files remaining"
fi

echo ""
echo "================================================"
echo "🎉 Database lock cleanup completed for: $DB_NAME"
echo ""
echo "💡 Tips:"
echo "   - If issues persist, try restarting your terminal"
echo "   - Check file permissions: ls -la $DB_NAME"
echo "   - Ensure no other applications are using the database"
echo "" 